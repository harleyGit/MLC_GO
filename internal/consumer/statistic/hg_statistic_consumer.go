package statistic

import (
	"MLC_GO/internal/events"
	VideoEventsPackage "MLC_GO/internal/events/video"
	"context"
	"fmt"
)

// Counter 定义统计读模型的原子幂等计数边界。
type Counter interface {
	Increment(ctx context.Context, eventID string, metric string) error
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
	if err := c.counter.Increment(ctx, envelope.EventID, envelope.EventName); err != nil {
		return fmt.Errorf("increment statistic projection: %w", err)
	}
	return nil
}
