package VideoUploadCachePackage

import (
	VideoUploadDtoPackage "MLC_GO/internal/modules/video_upload/dto"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	uploadSessionTTL      = 24 * time.Hour
	submitIdempotencyTTL  = 10 * time.Minute
	rateLimitWindow       = time.Minute
	videoListPageTTL      = 5 * time.Second
	userUploadMinuteLimit = 120
	ipUploadMinuteLimit   = 600
	rateLimitRequestCost  = 1
)

var (
	ErrRateLimited     = errors.New("上传请求过于频繁")
	ErrDuplicateSubmit = errors.New("稿件正在提交，请勿重复操作")
)

// Cache 封装视频上传需要的 Redis 能力：上传会话、限流和提交幂等。
// 这些状态天然是短生命周期、跨实例共享的数据，适合放 Redis，而不是放进 MySQL 或进程内存。
type Cache struct {
	redisService *PersistenceRedisPackage.RedisService
	client       *redis.Client
}

// NewCache 创建视频上传缓存组件。
func NewCache(redisService *PersistenceRedisPackage.RedisService) *Cache {
	return &Cache{
		redisService: redisService,
		client:       redisService.Client(),
	}
}

// SaveUploadSession 记录用户与稿件的上传会话，便于后续做断点续传、状态恢复和防串稿。
func (c *Cache) SaveUploadSession(ctx context.Context, userID string, submissionID string) error {
	key := sessionKey(userID, submissionID)
	return c.redisService.SetToRedisV2(key, map[string]string{
		"userId":       userID,
		"submissionId": submissionID,
	}, uploadSessionTTL, ctx)
}

// TouchUploadSession 延长上传会话生命周期，适合多 P 连续上传场景。
func (c *Cache) TouchUploadSession(ctx context.Context, userID string, submissionID string) error {
	// 给 Redis 的 Key 设置过期时间（TTL），到时间后 Redis 自动删除该 Key
	return c.client.Expire(ctx, sessionKey(userID, submissionID), uploadSessionTTL).Err()
}

// CheckUploadRateLimit 对用户维度和 IP 维度同时限流。
// Lua 脚本保证 INCR 和 EXPIRE 原子执行，避免进程异常导致 key 永久不过期。
func (c *Cache) CheckUploadRateLimit(ctx context.Context, userID string, ip string) error {
	if err := c.checkLimit(ctx, userRateKey(userID), userUploadMinuteLimit); err != nil {
		return err
	}
	return c.checkLimit(ctx, ipRateKey(ip), ipUploadMinuteLimit)
}

// AcquireSubmitLock 获取稿件提交幂等锁。
// 同一个用户同一个 submissionId 在短时间内只能有一个保存/提交请求进入写库链路。
// 返回锁标识（UUID），用于安全释放锁；如果获取失败返回空字符串。
func (c *Cache) AcquireSubmitLock(ctx context.Context, userID string, submissionID string) (string, error) {
	// 生成 UUID 作为锁的标识，防止误删其他请求持有的锁
	lockValue := uuid.NewString()
	// 用于分布式锁，SetNX 只有在 key 不存在时才设置成功，成功时返回 true；如果 key 已存在，说明已有请求持有锁，返回 false。
	ok, err := c.client.SetNX(ctx, submitLockKey(userID, submissionID), lockValue, submitIdempotencyTTL).Result()
	if err != nil {
		return "", err
	}
	if !ok {
		return "", nil
	}
	return lockValue, nil
}

// ReleaseSubmitLock 安全释放提交幂等锁。
// 使用 Lua 脚本校验锁的 value 是否匹配，匹配才删除，防止误删其他请求持有的锁。
func (c *Cache) ReleaseSubmitLock(ctx context.Context, userID string, submissionID string, lockValue string) error {
	// 如果锁标识为空，说明没有获取到锁，直接返回
	if lockValue == "" {
		return nil
	}
	// 使用 Lua 脚本安全释放锁
	result, err := c.client.Eval(ctx, PersistenceRedisPackage.ReleaseSubmitLockLuaScript, []string{submitLockKey(userID, submissionID)}, lockValue).Int64()
	if err != nil {
		return err
	}
	// 如果 result 为 0，说明锁的 value 不匹配或已过期，忽略
	_ = result
	return nil
}

// SaveSubmitResult 缓存提交结果，后续可用于前端重复点击时快速返回上一次结果。
func (c *Cache) SaveSubmitResult(ctx context.Context, userID string, submissionID string, status string) error {
	return c.redisService.SetToRedisV2(submitResultKey(userID, submissionID), map[string]string{
		"submissionId": submissionID,
		"status":       status,
	}, submitIdempotencyTTL, ctx)
}

// checkLimit 对单个 Redis key 执行令牌桶限流。
// Eval 会把 Lua 脚本发送到 Redis 服务端执行，Redis 执行脚本时具备原子性：脚本执行期间不会插入其他命令。
// 参数说明：
// - ctx：沿用请求上下文，调用方取消或超时时 Redis 命令也能退出。
// - TokenBucketRateLimitLuaScript：要在 Redis 服务端执行的令牌桶 Lua 脚本，集中定义在 Redis 基础设施包。
// - []string{key}：传给 Lua 的 KEYS，脚本里通过 KEYS[1] 读取。
// - capacity：传给 Lua 的 ARGV[1]，桶容量，决定允许的最大短时 burst。
// - refillRate：传给 Lua 的 ARGV[2]，每秒补充多少 token。
// - now_ms：传给 Lua 的 ARGV[3]，当前毫秒时间戳。
// - requested：传给 Lua 的 ARGV[4]，本次请求消耗 token 数。
// - ttl：传给 Lua 的 ARGV[5]，桶状态多久不用后自动过期。
// 示例：用户 1 分钟限 120 次时，refillRate=2 token/s；请求突刺会消耗桶内 token，耗尽后只能按 2 次/秒继续通过。
func (c *Cache) checkLimit(ctx context.Context, key string, limit int64) error {
	capacity := limit                                           //令牌桶最大容量 = 120
	refillRate := float64(limit) / rateLimitWindow.Seconds()    //每秒补充2个token; rateLimitWindow 是一分钟，加上Seconds() 就是60秒，120/60=2
	nowMillis := time.Now().UnixMilli()                         //获取当前时间戳（毫秒）。
	ttlSeconds := int(math.Ceil(rateLimitWindow.Seconds() * 2)) // rateLimitWindow.Seconds()1分钟是60秒，乘以2就是120秒，向上取整就是120秒，也就是2分钟。这个 TTL 是为了在没有请求时自动清理 Redis 中的限流状态，避免占用过多内存。

	allowed, err := c.client.Eval(ctx, PersistenceRedisPackage.TokenBucketRateLimitLuaScript, []string{key}, capacity, refillRate, nowMillis, rateLimitRequestCost, ttlSeconds).Int64()
	if err != nil {
		return err
	}
	if allowed != 1 {
		return ErrRateLimited
	}
	return nil
}

func sessionKey(userID string, submissionID string) string {
	return fmt.Sprintf("%s%s:%s", PersistenceRedisPackage.VideoUploadSessionKeyPrefix, userID, submissionID)
}

func userRateKey(userID string) string {
	return fmt.Sprintf("%s%s", PersistenceRedisPackage.VideoUploadUserRateKeyPrefix, userID)
}

func ipRateKey(ip string) string {
	return fmt.Sprintf("%s%s", PersistenceRedisPackage.VideoUploadIPRateKeyPrefix, ip)
}

func submitLockKey(userID string, submissionID string) string {
	return fmt.Sprintf("%s%s:%s", PersistenceRedisPackage.VideoUploadSubmitLockKeyPrefix, userID, submissionID)
}

func submitResultKey(userID string, submissionID string) string {
	return fmt.Sprintf("%s%s:%s", PersistenceRedisPackage.VideoUploadSubmitResultKeyPrefix, userID, submissionID)
}

func videoListPageKey(cursor string, pageSize int) string {
	if cursor == "" {
		cursor = "first"
	}
	return fmt.Sprintf("%s%s:size:%d", PersistenceRedisPackage.VideoUploadListPageKeyPrefix, cursor, pageSize)
}

func videoStatusCounterKey() string {
	return PersistenceRedisPackage.VideoStatusCounterKey
}

// GetInt 从 Redis 获取整数值，key 不存在时返回 -1。
func (c *Cache) GetInt(ctx context.Context, key string) (int, error) {
	val, err := c.client.Get(ctx, key).Int()
	if errors.Is(err, redis.Nil) {
		return -1, nil
	}
	return val, err
}

// SetInt 向 Redis 写入整数值并设置 TTL。
func (c *Cache) SetInt(ctx context.Context, key string, value int, ttl time.Duration) error {
	return c.client.Set(ctx, key, value, ttl).Err()
}

// IncrementVideoStatusCounter 原子调整某个稿件状态的计数。
// Redis HINCRBY 是单命令原子操作，适合 10k~100k+ QPS 的列表总数热点查询写侧维护。
func (c *Cache) IncrementVideoStatusCounter(ctx context.Context, status string, delta int64) error {
	if delta == 0 || status == "" {
		return nil
	}
	//HIncrBy是redis中操作 Redis Hash 类型字段自增的方法。
	return c.client.HIncrBy(ctx, videoStatusCounterKey(), status, delta).Err()
}

// GetVideoStatusCounters 从 Redis Hash 读取所有状态计数。
// 返回 hit=false 表示计数器尚未初始化，调用方应回源 MySQL 并回填。
func (c *Cache) GetVideoStatusCounters(ctx context.Context) (map[string]int64, bool, error) {
	// 返回所有状态计数。
	values, err := c.client.HGetAll(ctx, videoStatusCounterKey()).Result()
	if err != nil {
		return nil, false, err
	}
	if len(values) == 0 {
		return nil, false, nil
	}

	counters := make(map[string]int64, len(values))
	for status, value := range values {
		count, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nil, false, err
		}
		counters[status] = count
	}
	return counters, true, nil
}

// SetVideoStatusCounters 用 MySQL 精确回源结果初始化 Redis 计数器。
// 计数器不设置 TTL，避免高并发下 key 过期瞬间把热点请求打回 MySQL；一致性由写侧 HINCRBY 和后续补偿任务保证。
func (c *Cache) SetVideoStatusCounters(ctx context.Context, counters map[string]int64) error {
	if len(counters) == 0 {
		return nil
	}
	values := make(map[string]interface{}, len(counters))
	for status, count := range counters {
		values[status] = count
	}
	return c.client.HSet(ctx, videoStatusCounterKey(), values).Err()
}

// GetVideoListPage 读取视频列表页缓存。
// 列表页是高 QPS 热点读路径，短 TTL 缓存用于吸收瞬时并发；miss 时仍回源 MySQL 游标分页。
func (c *Cache) GetVideoListPage(ctx context.Context, cursor string, pageSize int) (*VideoUploadDtoPackage.GetVideoListResponse, bool, error) {
	data, err := c.client.Get(ctx, videoListPageKey(cursor, pageSize)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	var resp VideoUploadDtoPackage.GetVideoListResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, false, err
	}
	return &resp, true, nil
}

// SetVideoListPage 写入视频列表页短 TTL 缓存。
func (c *Cache) SetVideoListPage(ctx context.Context, cursor string, pageSize int, resp *VideoUploadDtoPackage.GetVideoListResponse) error {
	if resp == nil {
		return nil
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, videoListPageKey(cursor, pageSize), data, videoListPageTTL).Err()
}

// InvalidateVideoListPages 清理列表分页缓存。
// 写侧提交审核后调用；SCAN 分批删除，避免 KEYS 在生产 Redis 上阻塞事件循环。
func (c *Cache) InvalidateVideoListPages(ctx context.Context) error {
	var cursor uint64
	for {
		// 在不阻塞 Redis 的情况下，渐进式遍历数据库中的 Key。对于亿级 Key 的 Redis 集群，这是唯一推荐的遍历方式。
		// TODO：对于在线高 QPS 业务查询，则应该避免依赖 Scan()，改用预先设计好的索引结构。
		keys, nextCursor, err := c.client.Scan(ctx, cursor, PersistenceRedisPackage.VideoUploadListPagePatternKey, 100).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := c.client.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		if nextCursor == 0 {
			return nil
		}
		cursor = nextCursor
	}
}
