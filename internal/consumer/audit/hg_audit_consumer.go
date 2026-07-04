package audit

import (
	"MLC_GO/internal/events"
	VideoEventsPackage "MLC_GO/internal/events/video"
	"context"
)

// Consumer 处理审核流事件。
type Consumer struct{}

// NewConsumer 创建审核事件消费者。
func NewConsumer() *Consumer { return &Consumer{} }

// Handle 处理审核流关注的视频审核事件。
func (c *Consumer) Handle(ctx context.Context, envelope events.EventEnvelope) error {
	if envelope.EventName == VideoEventsPackage.VideoReviewedEventName {
		// TODO: 投递审核系统或写入审核队列。
		return nil
	}
	// 非审核事件直接忽略，避免跨业务事件互相影响。
	return nil
}
