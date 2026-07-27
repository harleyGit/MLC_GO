package statistic

import (
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"context"
	"fmt"
)

const (
	hgDefaultStatisticShardCount = 64
	hgDefaultStatisticGeneration = "v2"
)

// RedisEvalClient 是统计原子计数所需的最小 Redis Lua 接口。
type RedisEvalClient interface {
	Eval(ctx context.Context, script string, keys []string, args ...any) error
}

// RedisCounter 使用 Lua 原子完成 EventID 去重和 Hash 计数。
type RedisCounter struct {
	client     RedisEvalClient
	shardCount int
	generation string
}

// NewRedisCounter 创建 Redis 统计计数器。
func NewRedisCounter(client RedisEvalClient, shardCount int, generation string) *RedisCounter {
	if shardCount <= 0 {
		shardCount = hgDefaultStatisticShardCount
	}
	if generation == "" {
		generation = hgDefaultStatisticGeneration
	}
	return &RedisCounter{client: client, shardCount: shardCount, generation: generation}
}

// Increment 原子去重后将事件名对应计数加一。
func (c *RedisCounter) Increment(ctx context.Context, delivery Delivery, eventID string, metric string) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("statistic redis client cannot be nil")
	}
	shard := PersistenceRedisPackage.GetStatisticShard(delivery.Partition, c.shardCount)
	keys := []string{PersistenceRedisPackage.GetVideoEventCounterKey(c.generation, shard), PersistenceRedisPackage.GetVideoStatisticOffsetWatermarkKey(c.generation, shard)}
	return c.client.Eval(ctx, PersistenceRedisPackage.VideoStatisticIncrementLuaScript, keys, metric, fmt.Sprintf("%s:%d", delivery.Topic, delivery.Partition), delivery.Offset)
}
