package VideoCommentCachePackage

import (
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"context"
	"errors"
	"math"
	"time"
)

var ErrImageRateLimited = errors.New("评论图片上传过于频繁")

type hgRateEval interface {
	EvalInt64(context.Context, string, []string, ...any) (int64, error)
}

type hgRedisRateEval struct {
	redis *PersistenceRedisPackage.RedisService
}

func (e hgRedisRateEval) EvalInt64(ctx context.Context, script string, keys []string, args ...any) (int64, error) {
	return e.redis.Client().Eval(ctx, script, keys, args...).Int64()
}

// HGImageRateLimitConfig bounds per-user and per-source-IP upload bursts.
type HGImageRateLimitConfig struct {
	UserCapacity int64
	IPCapacity   int64
	Window       time.Duration
}

// HGImageRateLimiter applies two fail-closed Redis token buckets before storage I/O.
type HGImageRateLimiter struct {
	eval   hgRateEval
	config HGImageRateLimitConfig
	now    func() time.Time
}

// NewHGImageRateLimiter 创建可注入 Redis Eval 实现的限流器，主要供模块装配和单元测试使用。
func NewHGImageRateLimiter(eval hgRateEval, config HGImageRateLimitConfig) (*HGImageRateLimiter, error) {
	if eval == nil || config.UserCapacity < 1 || config.IPCapacity < 1 || config.Window <= 0 {
		return nil, errors.New("comment image rate limit configuration is invalid")
	}
	return &HGImageRateLimiter{eval: eval, config: config, now: time.Now}, nil
}

// NewHGRedisImageRateLimiter 复用应用 Redis 连接池；Redis 不可用时拒绝构造，上传链路不会无保护放行。
func NewHGRedisImageRateLimiter(redis *PersistenceRedisPackage.RedisService, config HGImageRateLimitConfig) (*HGImageRateLimiter, error) {
	if redis == nil || redis.Client() == nil {
		return nil, errors.New("redis unavailable")
	}
	return NewHGImageRateLimiter(hgRedisRateEval{redis: redis}, config)
}

// Allow checks user first and source IP second; Redis errors fail closed before object storage work starts.
func (l *HGImageRateLimiter) Allow(ctx context.Context, userID, sourceIP string) error {
	checks := []struct {
		key      string
		capacity int64
	}{
		{PersistenceRedisPackage.VideoCommentImageUserRateKeyPrefix + userID, l.config.UserCapacity},
		{PersistenceRedisPackage.VideoCommentImageIPRateKeyPrefix + sourceIP, l.config.IPCapacity},
	}
	for _, check := range checks {
		allowed, err := l.eval.EvalInt64(ctx, PersistenceRedisPackage.TokenBucketRateLimitLuaScript, []string{check.key}, check.capacity, float64(check.capacity)/l.config.Window.Seconds(), l.now().UnixMilli(), 1, int(math.Ceil(l.config.Window.Seconds()*2)))
		if err != nil {
			return err
		}
		if allowed != 1 {
			return ErrImageRateLimited
		}
	}
	return nil
}
