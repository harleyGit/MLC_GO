/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-07-25 21:15:01
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-08-26 17:55:37
 * @FilePath: /MLC_GO/internal/consumer/feed/hg_redis_projector.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
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

// 向 Redis 分片 Feed 流写入一条投稿 (submissionID)，基于 score 做有序集合，维护偏移量水位，做幂等去重，并且限制 Feed 最大条目，超量淘汰旧数据
//
//	@param ctx
//	@param delivery
//	@param eventID
//	@param submissionID
//	@param score
//	@return error
func (p *RedisProjector) Publish(ctx context.Context, delivery Delivery, eventID string, submissionID string, score int64) error {
	if p == nil || p.client == nil {
		return fmt.Errorf("feed redis client cannot be nil")
	}
	// 1. 根据 submissionID 计算落到哪个分片
	shard := PersistenceRedisPackage.GetFeedShard(submissionID, p.shardCount)
	// 2. 组装2个redis key
	// KEYS[1]: 当前分片Feed有序集合 zset
	// KEYS[2]: 当前分片offset水位hash表
	keys := []string{PersistenceRedisPackage.GetFeedPublishedZSetKey(p.generation, shard), PersistenceRedisPackage.GetFeedOffsetWatermarkKey(p.generation, shard)}
	// 调用Eval执行Lua脚本，把keys、多个参数传给lua
	return p.client.Eval(
		ctx,
		PersistenceRedisPackage.FeedPublishLuaScript,
		keys,                               // KEYS数组，lua中用KEYS[1],KEYS[2]访问； KEYS[1]：Feed ZSet 集合 key，存储 submissionID，score 排序 ；KEYS[2] Hash 表 key，存 offset 水位标记
		submissionID,                       // ARGV[1] 投稿 ID，zset member
		score,                              // ARGV[2] zset的score； zset 排序分数，一般是时间戳 / 排序权重
		p.maxItems,                         // ARGV[3] feed最大保留条数
		hgDeliveryWatermarkField(delivery), // ARGV[4] hash的field名称
		delivery.Offset,                    // ARGV[5] 当前这条消息的offset数值
	)
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
