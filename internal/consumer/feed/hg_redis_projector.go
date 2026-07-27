package feed

import (
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"context"
	"fmt"
)

const (
	hgDefaultFeedMaxItems       = 100000
	hgDefaultFeedIdempotencyTTL = int64(30 * 24 * 60 * 60)
)

// RedisEvalClient 是 Feed 原子投影所需的最小 Redis Lua 接口。
type RedisEvalClient interface {
	Eval(ctx context.Context, script string, keys []string, args ...any) error
}

// RedisProjector 使用单条 Lua 原子维护 Feed ZSET、容量上限和 EventID 幂等标记。
type RedisProjector struct {
	client         RedisEvalClient
	maxItems       int
	idempotencyTTL int64
}

// NewRedisProjector 创建 Redis Feed 投影器。
func NewRedisProjector(client RedisEvalClient, maxItems int, idempotencyTTLSeconds int64) *RedisProjector {
	if maxItems <= 0 {
		maxItems = hgDefaultFeedMaxItems
	}
	if idempotencyTTLSeconds <= 0 {
		idempotencyTTLSeconds = hgDefaultFeedIdempotencyTTL
	}
	return &RedisProjector{client: client, maxItems: maxItems, idempotencyTTL: idempotencyTTLSeconds}
}

// Publish 原子写入已发布视频并裁剪最旧成员，避免全局 ZSET 无界增长。
func (p *RedisProjector) Publish(ctx context.Context, eventID string, submissionID string, score int64) error {
	if p == nil || p.client == nil {
		return fmt.Errorf("feed redis client cannot be nil")
	}
	keys := []string{PersistenceRedisPackage.FeedPublishedZSetKey, PersistenceRedisPackage.GetFeedIdempotencyKey(eventID)}
	return p.client.Eval(ctx, PersistenceRedisPackage.FeedPublishLuaScript, keys, submissionID, score, p.maxItems, p.idempotencyTTL)
}

// Delete 原子去重并移除已删除或下架视频。
func (p *RedisProjector) Delete(ctx context.Context, eventID string, submissionID string) error {
	if p == nil || p.client == nil {
		return fmt.Errorf("feed redis client cannot be nil")
	}
	keys := []string{PersistenceRedisPackage.FeedPublishedZSetKey, PersistenceRedisPackage.GetFeedIdempotencyKey(eventID)}
	return p.client.Eval(ctx, PersistenceRedisPackage.FeedDeleteLuaScript, keys, submissionID, p.idempotencyTTL)
}
