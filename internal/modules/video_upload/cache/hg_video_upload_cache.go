package VideoUploadCachePackage

import (
	PersistenceRedisPackage "MLC_GO/internal/infrastructure/persistence/redis"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	uploadSessionTTL      = 24 * time.Hour
	submitIdempotencyTTL  = 10 * time.Minute
	rateLimitWindow       = time.Minute
	userUploadMinuteLimit = 120
	ipUploadMinuteLimit   = 600
	sessionKeyPrefix      = "video_upload:session:"
	userRateKeyPrefix     = "video_upload:rate:user:"
	ipRateKeyPrefix       = "video_upload:rate:ip:"
	submitLockKeyPrefix   = "video_upload:submit_lock:"
	submitResultKeyPrefix = "video_upload:submit_result:"
	rateLimitLuaScript    = `
local current = redis.call('INCR', KEYS[1])
if current == 1 then
  redis.call('EXPIRE', KEYS[1], ARGV[2])
end
return current
`
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
func (c *Cache) AcquireSubmitLock(ctx context.Context, userID string, submissionID string) (bool, error) {
	return c.client.SetNX(ctx, submitLockKey(userID, submissionID), "1", submitIdempotencyTTL).Result()
}

// ReleaseSubmitLock 释放提交幂等锁。
func (c *Cache) ReleaseSubmitLock(ctx context.Context, userID string, submissionID string) error {
	return c.client.Del(ctx, submitLockKey(userID, submissionID)).Err()
}

// SaveSubmitResult 缓存提交结果，后续可用于前端重复点击时快速返回上一次结果。
func (c *Cache) SaveSubmitResult(ctx context.Context, userID string, submissionID string, status string) error {
	return c.redisService.SetToRedisV2(submitResultKey(userID, submissionID), map[string]string{
		"submissionId": submissionID,
		"status":       status,
	}, submitIdempotencyTTL, ctx)
}

func (c *Cache) checkLimit(ctx context.Context, key string, limit int64) error {
	count, err := c.client.Eval(ctx, rateLimitLuaScript, []string{key}, limit, int(rateLimitWindow.Seconds())).Int64()
	if err != nil {
		return err
	}
	if count > limit {
		return ErrRateLimited
	}
	return nil
}

func sessionKey(userID string, submissionID string) string {
	return fmt.Sprintf("%s%s:%s", sessionKeyPrefix, userID, submissionID)
}

func userRateKey(userID string) string {
	return fmt.Sprintf("%s%s", userRateKeyPrefix, userID)
}

func ipRateKey(ip string) string {
	return fmt.Sprintf("%s%s", ipRateKeyPrefix, ip)
}

func submitLockKey(userID string, submissionID string) string {
	return fmt.Sprintf("%s%s:%s", submitLockKeyPrefix, userID, submissionID)
}

func submitResultKey(userID string, submissionID string) string {
	return fmt.Sprintf("%s%s:%s", submitResultKeyPrefix, userID, submissionID)
}
