package feed

import (
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"context"
	"fmt"
)

const (
	hgDefaultFeedShardCount = 64
	hgDefaultFeedMaxItems   = 2000
	hgDefaultFeedGeneration = "v2"
)

// RedisEvalClient 是 Feed 原子投影所需的最小 Redis Lua 接口。
type RedisEvalClient interface {
	Eval(ctx context.Context, script string, keys []string, args ...any) error
}

// RedisProjector 使用单条 Lua 原子维护 Feed ZSET、容量上限和 EventID 幂等标记。
type RedisProjector struct {
	client     RedisEvalClient
	shardCount int
	maxItems   int
	generation string
}

// NewRedisProjector 创建 Redis Feed 投影器。
func NewRedisProjector(client RedisEvalClient, shardCount int, maxItems int, generation string) *RedisProjector {
	if shardCount <= 0 {
		shardCount = hgDefaultFeedShardCount
	}
	if maxItems <= 0 {
		maxItems = hgDefaultFeedMaxItems
	}
	if generation == "" {
		generation = hgDefaultFeedGeneration
	}
	return &RedisProjector{client: client, shardCount: shardCount, maxItems: maxItems, generation: generation}
}

// Publish 原子写入已发布视频并裁剪最旧成员，避免全局 ZSET 无界增长。
func (p *RedisProjector) Publish(ctx context.Context, delivery Delivery, eventID string, submissionID string, score int64) error {
	if p == nil || p.client == nil {
		return fmt.Errorf("feed redis client cannot be nil")
	}
	shard := PersistenceRedisPackage.GetFeedShard(submissionID, p.shardCount)
	keys := []string{PersistenceRedisPackage.GetFeedPublishedZSetKey(p.generation, shard), PersistenceRedisPackage.GetFeedOffsetWatermarkKey(p.generation, shard)}
	return p.client.Eval(ctx, PersistenceRedisPackage.FeedPublishLuaScript, keys, submissionID, score, p.maxItems, hgDeliveryWatermarkField(delivery), delivery.Offset)
}

// Delete 原子去重并移除已删除或下架视频。
func (p *RedisProjector) Delete(ctx context.Context, delivery Delivery, eventID string, submissionID string) error {
	if p == nil || p.client == nil {
		return fmt.Errorf("feed redis client cannot be nil")
	}
	shard := PersistenceRedisPackage.GetFeedShard(submissionID, p.shardCount)
	keys := []string{PersistenceRedisPackage.GetFeedPublishedZSetKey(p.generation, shard), PersistenceRedisPackage.GetFeedOffsetWatermarkKey(p.generation, shard)}
	return p.client.Eval(ctx, PersistenceRedisPackage.FeedDeleteLuaScript, keys, submissionID, hgDeliveryWatermarkField(delivery), delivery.Offset)
}

func hgDeliveryWatermarkField(delivery Delivery) string {
	return fmt.Sprintf("%s:%d", delivery.Topic, delivery.Partition)
}
