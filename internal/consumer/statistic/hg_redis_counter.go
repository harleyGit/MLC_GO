package statistic

import (
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"context"
	"fmt"
)

const hgDefaultStatisticIdempotencyTTL = int64(30 * 24 * 60 * 60)

// RedisEvalClient 是统计原子计数所需的最小 Redis Lua 接口。
type RedisEvalClient interface {
	Eval(ctx context.Context, script string, keys []string, args ...any) error
}

// RedisCounter 使用 Lua 原子完成 EventID 去重和 Hash 计数。
type RedisCounter struct {
	client         RedisEvalClient
	idempotencyTTL int64
}

// NewRedisCounter 创建 Redis 统计计数器。
func NewRedisCounter(client RedisEvalClient, idempotencyTTLSeconds int64) *RedisCounter {
	if idempotencyTTLSeconds <= 0 {
		idempotencyTTLSeconds = hgDefaultStatisticIdempotencyTTL
	}
	return &RedisCounter{client: client, idempotencyTTL: idempotencyTTLSeconds}
}

// Increment 原子去重后将事件名对应计数加一。
func (c *RedisCounter) Increment(ctx context.Context, eventID string, metric string) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("statistic redis client cannot be nil")
	}
	keys := []string{PersistenceRedisPackage.VideoEventCounterKey, PersistenceRedisPackage.GetVideoStatisticIdempotencyKey(eventID)}
	return c.client.Eval(ctx, PersistenceRedisPackage.VideoStatisticIncrementLuaScript, keys, metric, c.idempotencyTTL)
}
