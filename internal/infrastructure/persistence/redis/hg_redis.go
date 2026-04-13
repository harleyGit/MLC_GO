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
	HGLoggerPackage "MLC_GO/internal/logger"
	"MLC_GO/internal/pkg/logHG"
	"context"
	"encoding/json"
	"os"
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

func getRedisAddr() string {
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = "localhost"
	}

	redisPort := os.Getenv("REDIS_PORT")
	if redisPort == "" {
		redisPort = "6379"
	}

	return redisHost + ":" + redisPort
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
	return &RedisService{
		client: redis.NewClient(&redis.Options{
			Addr: getRedisAddr(),
			// Password: "", // no password set
			// DB:       0,  // use default DB
		}),
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
	redisServer := &RedisService{
		client: redis.NewClient(&redis.Options{
			Addr: getRedisAddr(),
			// Password: "", // no password set
			// DB:       0,  // use default DB
		}),
		defaultCtx: defaultCtx,
	}

	if err := redisServer.client.Ping(context.Background()).Err(); err != nil {
		logHG.FatalFInfo("redis 连接失败:", err)
		return nil
	}

	RDB = redisServer.client // 全局变量赋值
	return redisServer
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
	return DeleteFromRedis(key, WithContext(ctx))
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
