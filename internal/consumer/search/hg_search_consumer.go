package search

import (
	"MLC_GO/internal/consumer"
	"MLC_GO/internal/events"
	VideoEventsPackage "MLC_GO/internal/events/video"
	"context"
)

// Consumer 维护搜索索引。
// 发布/删除事件最终应同步到 ES/OpenSearch，接口读路径避免扫 MySQL。
type Consumer struct{}

// NewConsumer 创建搜索索引事件消费者。
func NewConsumer() *Consumer { return &Consumer{} }

// Handle 处理搜索索引需要同步的视频发布/删除事件。
func (c *Consumer) Handle(ctx context.Context, envelope events.EventEnvelope) error {
	switch envelope.EventName {
	case VideoEventsPackage.VideoPublishedEventName, VideoEventsPackage.VideoDeletedEventName:
		return consumer.NewHandlerNotImplementedError("search", envelope.EventName)
	default:
		// 搜索索引不关心的事件不报错，避免阻塞同一 consumer group 的正常提交。
		return nil
	}
}
