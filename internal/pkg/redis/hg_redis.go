/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-13 21:28:04
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-03-01 20:43:28
 * @FilePath: /MLC_GO/internal/cache/hg_redis.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package PersistenceRedisPackage

import (
	ConfigPackage "MLC_GO/internal/pkg/config"
	"MLC_GO/internal/pkg/logHG"
	HGLoggerPackage "MLC_GO/internal/pkg/logger"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisService struct {
	// Redis的Set(ctx, key, value, ttl)/Get(ctx, key)时的ctx作用
	// 1.控制请求生命周期
	// 2.超时/取消/链路追踪
	// 3. 不参与 Redis TCP连接池管理
	// 4. 不参与连接池的敷用
	// 6. Redis的连接池由redis.Client管理，与context是否创建无关。
	client     *redis.Client
	defaultCtx context.Context
}
type options struct {
	ctx        context.Context
	timeout    time.Duration
	maxRetries int
}
type RedisOption func(*options)

var (
	ctx = context.Background()
	RDB *redis.Client
)

const (
	// defaultRedisPoolSize 控制单实例 Redis 最大连接池大小。
	// 高并发下 Redis 客户端会复用这些连接，避免每个请求都建立 TCP 连接。
	defaultRedisPoolSize = 200
	// defaultRedisMinIdleConns 保持一定数量的预热空闲连接，降低突发流量的建连延迟。
	defaultRedisMinIdleConns = 20
	// defaultRedisMaxRetries 限制自动重试次数，避免 Redis 抖动时请求被无限放大。
	defaultRedisMaxRetries = 2
	// defaultRedisDialTimeout 限制建连耗时，Redis 不可达时快速失败。
	defaultRedisDialTimeout = 3 * time.Second
	// defaultRedisReadTimeout 限制单次读取耗时，避免慢 Redis 响应拖住业务 goroutine。
	defaultRedisReadTimeout = 2 * time.Second
	// defaultRedisWriteTimeout 限制单次写入耗时，避免网络异常时写操作长时间阻塞。
	defaultRedisWriteTimeout = 2 * time.Second
)

/* 比如：debug 环境中，Go 程序启动后加载 config/base 与 config/debug 下的模块配置。 */
func newRedisClient() (*redis.Client, error) {
	cfg, err := ConfigPackage.GetRedisConfig()
	if err != nil {
		return nil, fmt.Errorf("加载 Redis 配置失败: %w", err)
	}
	// Redis 客户端内部自带连接池，应用应该创建少量长期复用的 client，而不是每个请求创建。
	// 这些参数均可通过环境变量覆盖，便于按机器规格、Redis 集群规格和副本数调优。
	return redis.NewClient(&redis.Options{
		Addr:         cfg.Host + ":" + cfg.Port,
		PoolSize:     getEnvInt("REDIS_POOL_SIZE", defaultRedisPoolSize),
		MinIdleConns: getEnvInt("REDIS_MIN_IDLE_CONNS", defaultRedisMinIdleConns),
		MaxRetries:   getEnvInt("REDIS_MAX_RETRIES", defaultRedisMaxRetries),
		DialTimeout:  getEnvDuration("REDIS_DIAL_TIMEOUT", defaultRedisDialTimeout),
		ReadTimeout:  getEnvDuration("REDIS_READ_TIMEOUT", defaultRedisReadTimeout),
		WriteTimeout: getEnvDuration("REDIS_WRITE_TIMEOUT", defaultRedisWriteTimeout),
	}), nil
}

func WithContext(ctx context.Context) RedisOption {
	return func(o *options) {
		o.ctx = ctx
	}
}

func WithTimeout(timeout time.Duration) RedisOption {
	return func(o *options) {
		o.timeout = timeout
	}
}

func defaultRedisOptions() *options {
	return &options{
		ctx:        context.Background(),
		timeout:    5 * time.Second,
		maxRetries: 3,
	}
}

/*
	 NewRedisServiceV2使用范例
		// 使用默认值
		client1 := NewRedisServiceV2()

		// 传入自定义Context
		customCtx := context.WithValue(context.Background(), "trace-id", "abc123")
		client2 := NewRedisServiceV2(WithContext(customCtx))

		// 传入多个配置
		client3 := NewRedisServiceV2(
			WithContext(customCtx),
			WithTimeout(10*time.Second),
		)
*/
func NewRedisServiceV2(opts ...RedisOption) *RedisService {

	// 设置默认值
	options := defaultRedisOptions()
	// 应用传入选项
	for _, opt := range opts {
		opt(options)
	}
	// 使用opt构建RedisService
	// TODO: 若是初始化用 context.Background(), 则后续调用比如用Get传入的是http的
	// TODO: r *http.Request; r.Context()， 这样会不会冲突？
	client, err := newRedisClient()
	if err != nil {
		logHG.ErrFInfo("Redis 配置加载失败: %v", err)
		return nil
	}
	return &RedisService{
		client:     client,
		defaultCtx: options.ctx,
	}
}

/*
	 NewRedisService 使用范例：
		// 创建service时指定默认Context
	    svc1 := NewRedisService()  // 默认使用Background

	    customCtx := context.WithValue(context.Background(), "app", "myapp")
	    svc2 := NewRedisService(customCtx)  // 使用自定义默认Context

	    // 调用方法
	    svc1.Get("key1")  // 使用默认Background

	    // 临时使用不同的Context
	    reqCtx := context.WithValue(context.Background(), "request", "123")
	    svc2.Get("key2", reqCtx)  // 使用传入的Context
*/
func NewRedisService(ctx ...context.Context) *RedisService {
	var defaultCtx context.Context

	// 如果有传入Context,使用传入的； 否则使用Backgournd
	if len(ctx) > 0 && ctx[0] != nil {
		defaultCtx = ctx[0]
	} else {
		defaultCtx = context.Background()
	}
	client, err := newRedisClient()
	if err != nil {
		logHG.ErrFInfo("Redis 配置加载失败: %v", err)
		return nil
	}
	redisServer := &RedisService{
		client:     client,
		defaultCtx: defaultCtx,
	}

	if err := redisServer.client.Ping(context.Background()).Err(); err != nil {
		logHG.FatalFInfo("redis 连接失败:", err)
		return nil
	}

	RDB = redisServer.client // 全局变量赋值
	return redisServer
}

// NewRedisServiceWithError 初始化 Redis 连接并返回错误，供服务入口统一处理启动失败。
// 与旧 NewRedisService 不同，这里不直接 Fatal 退出进程，便于入口统一释放已初始化资源。
func NewRedisServiceWithError(ctx ...context.Context) (*RedisService, error) {
	var defaultCtx context.Context
	if len(ctx) > 0 && ctx[0] != nil {
		defaultCtx = ctx[0]
	} else {
		defaultCtx = context.Background()
	}

	client, err := newRedisClient()
	if err != nil {
		return nil, err
	}
	redisServer := &RedisService{
		client:     client,
		defaultCtx: defaultCtx,
	}

	if err := redisServer.client.Ping(defaultCtx).Err(); err != nil {
		return nil, err
	}

	RDB = redisServer.client
	return redisServer, nil
}

// Close 关闭 Redis 客户端连接池，供服务优雅关闭时调用。
// Redis client 持有连接池和后台状态，显式关闭可以避免测试泄漏和发布时资源残留。
func (redisService *RedisService) Close() error {
	if redisService == nil || redisService.client == nil {
		return nil
	}
	return redisService.client.Close()
}

// PingContext 检查 Redis 连接是否可用，供 readyz 使用。
// 使用调用方传入的 context，可以给 /readyz 设置独立超时，不让 Redis 抖动拖慢探活链路。
func (redisService *RedisService) PingContext(ctx context.Context) error {
	if redisService == nil || redisService.client == nil {
		return redis.Nil
	}
	return redisService.client.Ping(ctx).Err()
}

// Client 返回 Redis 底层客户端，只允许 service 层在必须使用 go-redis 原生命令时调用。
// 常规业务读写应优先使用 RedisService 的封装方法，避免模块代码散落全局 RDB 依赖。
func (redisService *RedisService) Client() *redis.Client {
	if redisService == nil {
		return nil
	}
	return redisService.client
}

// Eval 执行集中维护的 Lua 脚本，供需要多命令原子性的读模型、限流和锁逻辑使用。
func (redisService *RedisService) Eval(ctx context.Context, script string, keys []string, args ...any) error {
	if redisService == nil || redisService.client == nil {
		return redis.Nil
	}
	return redisService.client.Eval(ctx, script, keys, args...).Err()
}

func getEnvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err == nil && parsed > 0 {
		return parsed
	}
	seconds, err := strconv.Atoi(value)
	if err != nil || seconds <= 0 {
		return fallback
	}
	return time.Duration(seconds) * time.Second
}

func newRedis(addr string, password string, db int) {
	RDB = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password, // no password set
		DB:       db,       // use default DB
	})
}

func (redisService *RedisService) SetToRedisV2(key string, value interface{}, ttl time.Duration, ctx ...context.Context) error {
	var defaultContext context.Context
	if len(ctx) > 0 && ctx[0] != nil {
		defaultContext = ctx[0]
	} else {
		defaultContext = redisService.defaultCtx
	}
	bytes, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return redisService.client.Set(defaultContext, key, bytes, ttl).Err()
}
func SetToRedis(key string, value interface{}, ttl time.Duration, opts ...RedisOption) error {
	// 默认配置
	opt := defaultRedisOptions()
	// 应用所有传入的选项
	for _, option := range opts {
		option(opt)
	}
	bytes, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return RDB.Set(opt.ctx, key, bytes, ttl).Err()
}

func (redisService *RedisService) GetFromRedisV2(key string, ctx ...context.Context) (string, error) {
	var defaultContext context.Context
	if len(ctx) > 0 && ctx[0] != nil {
		defaultContext = ctx[0]
	} else {
		defaultContext = redisService.defaultCtx
	}

	val, err := redisService.client.Get(defaultContext, key).Result()
	if err == redis.Nil {
		return val, nil // key不存在，返回空字符串和nil错误
	}
	return val, err
}

func (redisService *RedisService) DeleteFromRedis(key string, ctx context.Context) error {
	if redisService == nil || redisService.client == nil {
		return redis.Nil
	}
	return redisService.client.Del(ctx, key).Err()
}

// DeleteFromRedisByPattern 按 pattern 批量删除 key，统一使用注入的 Redis 客户端。
// 高并发业务写操作后清理列表缓存时使用 SCAN 分批删除，避免 KEYS 阻塞 Redis。
func (redisService *RedisService) DeleteFromRedisByPattern(pattern string, ctx context.Context) error {
	if redisService == nil || redisService.client == nil {
		return redis.Nil
	}

	var cursor uint64
	for {
		keys, nextCursor, err := redisService.client.Scan(ctx, cursor, pattern, 200).Result()
		if err != nil {
			return err
		}

		if len(keys) > 0 {
			if err = redisService.client.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return nil
}

func GetFromRedis(ctx context.Context, key string, opts ...RedisOption) (string, error) {
	HGLoggerPackage.LogInfo(ctx, "Redis Get:"+key)
	// 默认配置
	opt := defaultRedisOptions()
	// 应用所有传入的选项
	for _, option := range opts {
		option(opt)
	}
	return RDB.Get(opt.ctx, key).Result()
}

func DeleteFromRedis(key string, opts ...RedisOption) error {
	// 默认配置
	opt := defaultRedisOptions()
	// 应用所有传入的选项
	for _, option := range opts {
		option(opt)
	}
	return RDB.Del(opt.ctx, key).Err()
}

// DeleteFromRedisByPattern 批量删除匹配 pattern 的 Redis key。
func DeleteFromRedisByPattern(pattern string, opts ...RedisOption) error {
	opt := defaultRedisOptions()
	for _, option := range opts {
		option(opt)
	}

	var cursor uint64
	for {
		keys, nextCursor, err := RDB.Scan(opt.ctx, cursor, pattern, 200).Result()
		if err != nil {
			return err
		}

		if len(keys) > 0 {
			if err = RDB.Del(opt.ctx, keys...).Err(); err != nil {
				return err
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return nil
}
