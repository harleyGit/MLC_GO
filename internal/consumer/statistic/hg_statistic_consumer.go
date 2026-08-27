package statistic

import (
	"MLC_GO/internal/consumer"
	"MLC_GO/internal/events"
	VideoEventsPackage "MLC_GO/internal/events/video"
	ClickHousePackage "MLC_GO/internal/pkg/clickhouse"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"context"
	"encoding/json"
	"fmt"
)

// Delivery 是统计投影使用的 Kafka 来源坐标。
type Delivery = consumer.Delivery

// WithDelivery 注入 Kafka 来源坐标。
func WithDelivery(ctx context.Context, delivery Delivery) context.Context {
	return consumer.WithDelivery(ctx, delivery)
}

// Counter 定义统计读模型的原子幂等计数边界。
type Counter interface {
	Increment(ctx context.Context, delivery Delivery, eventID string, metric string) error
}

// EventStore 定义 Statistic 权威事件存储边界。
type EventStore interface {
	StoreStatisticEvent(ctx context.Context, event ClickHousePackage.HGStatisticEvent) error
}

// HGProjectionConfig 固定 Redis 投影 generation 与 shard 映射。
type HGProjectionConfig struct {
	RedisGeneration string
	RedisShardCount int
}

// Consumer 维护视频事件预聚合计数。
type Consumer struct {
	counter Counter
	store   EventStore
	config  HGProjectionConfig
}

// NewConsumer 创建统计事件消费者。
func NewConsumer(counter Counter, dependencies ...any) *Consumer {
	consumer := &Consumer{counter: counter, config: HGProjectionConfig{RedisGeneration: hgDefaultStatisticGeneration, RedisShardCount: hgDefaultStatisticShardCount}}
	// 把事件持久化存入 `EventStore`（底层一般是 ClickHouse，统计事件落库）
	for _, dependency := range dependencies {
		switch value := dependency.(type) {// 类型断言 switch（type‑switch）语法
		case EventStore:
			consumer.store = value //用来落事件到存储 (ClickHouse)
		case HGProjectionConfig: // 增量覆盖默认配置
			if value.RedisGeneration != "" { // 不为空才覆盖
				consumer.config.RedisGeneration = value.RedisGeneration
			}
			if value.RedisShardCount > 0 {// 大于 0 才覆盖
				consumer.config.RedisShardCount = value.RedisShardCount
			}
		}
	}
	return consumer
}

// Handle 按 EventID 原子去重后计数，避免至少一次投递重复累计。
// `Handle` 是 kafka 消费回调入口，收到一条事件 envelope 执行处理。
//	@param ctx 
//	@param envelope 
//	@return error 
func (c *Consumer) Handle(ctx context.Context, envelope events.EventEnvelope) error {

	// switch  只处理视频审核通过、发布、删除三类事件；其他事件直接返回 nil，跳过不处理。
	switch envelope.EventName {
	case VideoEventsPackage.VideoReviewedEventName, VideoEventsPackage.VideoPublishedEventName, VideoEventsPackage.VideoDeletedEventName:
	default:
		return nil
	}
	if c == nil || c.counter == nil {
		return fmt.Errorf("statistic counter cannot be nil")
	}
	if envelope.EventID == "" {
		return fmt.Errorf("statistic event id cannot be empty")
	}
	delivery, ok := consumer.DeliveryFromContext(ctx)
	if !ok || delivery.Topic == "" || delivery.Partition < 0 || delivery.Offset < 0 {
		return fmt.Errorf("statistic kafka delivery metadata is invalid")
	}
	if c.store != nil {
		var payload struct {
			SubmissionID string `json:"submissionId"`
			UserID       string `json:"userId"`
		}
		if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
			return fmt.Errorf("decode statistic event payload: %w", err)
		}
		if payload.SubmissionID == "" || payload.UserID == "" {
			return fmt.Errorf("statistic event submission id and user id cannot be empty")
		}
		event := ClickHousePackage.HGStatisticEvent{
			EventID: envelope.EventID, EventName: envelope.EventName, EventKey: envelope.EventKey,
			SubmissionID: payload.SubmissionID, UserID: payload.UserID, EventVersion: envelope.Version,
			EventTimestamp: envelope.Timestamp, SourceService: envelope.SourceService, TraceID: envelope.TraceID,
			RequestID: envelope.RequestID, KafkaTopic: delivery.Topic, KafkaPartition: delivery.Partition,
			KafkaOffset: delivery.Offset, RedisGeneration: c.config.RedisGeneration,
			RedisShard: PersistenceRedisPackage.GetStatisticShard(delivery.Partition, c.config.RedisShardCount), Payload: string(envelope.Payload),
		}

		// 确认点：StoreStatisticEvent 是否按 EventID 做了幂等？如果没有，这里就是一个重复写入隐患。理想做法是两者都幂等，或者用 outbox 模式。
		if err := c.store.StoreStatisticEvent(ctx, event); err != nil {
			hgStatisticAuthorityWriteFailure.Add(1) // 监控： 落库失败计数
			return fmt.Errorf("store statistic authority event: %w", err)
		}
		hgStatisticAuthorityWrites.Add(1) // 监控： 落库成功计数
	}
	// 调用 `counter.Increment()` 做 Redis 侧统计计数，依靠 `EventID` 做原子去重，解决 Kafka 至少一次投递带来的重复统计问题。
	if err := c.counter.Increment(ctx, delivery, envelope.EventID, envelope.EventName); err != nil {
		hgStatisticRedisFailures.Add(1)
		return fmt.Errorf("increment statistic projection: %w", err)
	}
	return nil
}
