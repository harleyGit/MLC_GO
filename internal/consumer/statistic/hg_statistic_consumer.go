package statistic

import (
	"MLC_GO/internal/consumer"
	"MLC_GO/internal/events"
	VideoEventsPackage "MLC_GO/internal/events/video"
	"context"
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

// Consumer 维护视频事件预聚合计数。
type Consumer struct {
	counter Counter
}

// NewConsumer 创建统计事件消费者。
func NewConsumer(counter Counter) *Consumer { return &Consumer{counter: counter} }

// Handle 按 EventID 原子去重后计数，避免至少一次投递重复累计。
func (c *Consumer) Handle(ctx context.Context, envelope events.EventEnvelope) error {
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
	if err := c.counter.Increment(ctx, delivery, envelope.EventID, envelope.EventName); err != nil {
		return fmt.Errorf("increment statistic projection: %w", err)
	}
	return nil
}
