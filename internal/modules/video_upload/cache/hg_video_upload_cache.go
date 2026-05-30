package VideoUploadCachePackage

import (
	PersistenceRedisPackage "MLC_GO/internal/infrastructure/persistence/redis"
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	uploadSessionTTL      = 24 * time.Hour
	submitIdempotencyTTL  = 10 * time.Minute
	rateLimitWindow       = time.Minute
	userUploadMinuteLimit = 120
	ipUploadMinuteLimit   = 600
	rateLimitRequestCost  = 1

	// releaseSubmitLockLua 是安全释放锁的 Lua 脚本。
	// 先校验锁的 value 是否匹配，匹配才删除，防止误删其他请求持有的锁。
	releaseSubmitLockLua = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
else
    return 0
end`
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
	result, err := c.client.Eval(ctx, releaseSubmitLockLua, []string{submitLockKey(userID, submissionID)}, lockValue).Int64()
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
	capacity := limit
	refillRate := float64(limit) / rateLimitWindow.Seconds()
	nowMillis := time.Now().UnixMilli()
	ttlSeconds := int(math.Ceil(rateLimitWindow.Seconds() * 2))

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
