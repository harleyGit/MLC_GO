package feed

import (
	"MLC_GO/internal/events"
	VideoEventsPackage "MLC_GO/internal/events/video"
	"context"
)

// Consumer 维护 Feed 读模型。
// 视频进入审核或发布后，可在这里写 Redis ZSET / 个性化推荐队列，列表接口不再高频打 MySQL。
type Consumer struct{}

// NewConsumer 创建 Feed 事件消费者。
func NewConsumer() *Consumer { return &Consumer{} }

// Handle 处理 Feed 读模型关心的视频事件。
func (c *Consumer) Handle(ctx context.Context, envelope events.EventEnvelope) error {
	switch envelope.EventName {
	case VideoEventsPackage.VideoReviewedEventName, VideoEventsPackage.VideoPublishedEventName:
		// TODO: 写入 feed:list 读模型。这里先保留骨架，避免接口请求路径直连 Kafka。
		return nil
	default:
		// 不属于 Feed 关注范围的事件直接忽略，保证同一 topic 可承载多类领域事件。
		return nil
	}
}
