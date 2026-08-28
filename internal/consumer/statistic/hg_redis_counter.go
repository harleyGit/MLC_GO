/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-07-27 09:36:37
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-08-27 21:08:06
 * @FilePath: /MLC_GO/internal/consumer/statistic/hg_redis_counter.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
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
	generation string // 版本代际，用于做数据重置、版本隔离；切换代际旧统计数据直接废弃，不用删 Redis。
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

// Increment Kafka 统计消费组 `statistic` 调用的核心计数方法
// 调用链路：`Consumer.Handle()` → 幂等校验 → `RedisCounter.Increment()`
//
//	@param ctx
//	@param delivery kafka 消息交付信息，包含 `Topic、Partition、Offset`（kafka 原生水位信息）
//	@param eventID 事件全局唯一 ID（EventEnvelope.EventID，用于幂等）
//	@param metric 指标名称，例如 `video:publish`、`video:like`，代表要统计哪一项业务指标。
//	@return error
func (c *RedisCounter) Increment(ctx context.Context, delivery Delivery, eventID string, metric string) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("statistic redis client cannot be nil")
	}
	shard := PersistenceRedisPackage.GetStatisticShard(delivery.Partition, c.shardCount)
	keys := []string{
		PersistenceRedisPackage.GetVideoEventCounterKey(c.generation, shard), // KEYS [1] = 事件计数器 key：分片内各个 metric 的计数存储
		PersistenceRedisPackage.GetVideoStatisticOffsetWatermarkKey(c.generation, shard), // KEYS [2] = offset 水位 key（watermark）：记录该分片已经处理到的 kafka offset。
	}
	return c.client.Eval(ctx, PersistenceRedisPackage.VideoStatisticIncrementLuaScript, keys, metric, fmt.Sprintf("%s:%d", delivery.Topic, delivery.Partition), delivery.Offset)
}
