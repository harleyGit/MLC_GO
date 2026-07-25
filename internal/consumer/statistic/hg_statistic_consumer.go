package statistic

import (
	"MLC_GO/internal/consumer"
	"MLC_GO/internal/events"
	VideoEventsPackage "MLC_GO/internal/events/video"
	"context"
)

// Consumer 维护统计读模型，如视频状态计数、发布量、审核量。
type Consumer struct{}

// NewConsumer 创建统计事件消费者。
func NewConsumer() *Consumer { return &Consumer{} }

// Handle 处理统计读模型关心的视频状态事件。
func (c *Consumer) Handle(ctx context.Context, envelope events.EventEnvelope) error {
	switch envelope.EventName {
	case VideoEventsPackage.VideoReviewedEventName, VideoEventsPackage.VideoPublishedEventName, VideoEventsPackage.VideoDeletedEventName:
		return consumer.NewHandlerNotImplementedError("statistic", envelope.EventName)
	default:
		// 统计模块只消费明确支持的事件，其余事件保持 no-op。
		return nil
	}
}
