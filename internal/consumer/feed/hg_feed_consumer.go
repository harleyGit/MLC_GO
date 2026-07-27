package feed

import (
	"MLC_GO/internal/consumer"
	"MLC_GO/internal/events"
	VideoEventsPackage "MLC_GO/internal/events/video"
	"context"
	"encoding/json"
	"fmt"
)

// Delivery 是 Feed 投影使用的 Kafka 来源坐标。
type Delivery = consumer.Delivery

// WithDelivery 供直接调用 Feed Handler 的测试和非 Kafka 适配器注入来源坐标。
func WithDelivery(ctx context.Context, delivery Delivery) context.Context {
	return consumer.WithDelivery(ctx, delivery)
}

// Projector 定义 Feed 读模型写入边界；实现必须按 EventID 原子幂等。
type Projector interface {
	Publish(ctx context.Context, delivery Delivery, eventID string, submissionID string, score int64) error
	Delete(ctx context.Context, delivery Delivery, eventID string, submissionID string) error
}

// Consumer 维护公开 Feed 读模型。
// 只有 video.published 才进入公开 Feed，进入审核的 video.reviewed 不得提前曝光。
type Consumer struct {
	projector Projector
}

// NewConsumer 创建 Feed 事件消费者。
func NewConsumer(projector Projector) *Consumer { return &Consumer{projector: projector} }

// Handle 处理视频发布和删除事件；投影失败必须返回错误，阻止 Kafka 提交 offset。
func (c *Consumer) Handle(ctx context.Context, envelope events.EventEnvelope) error {
	switch envelope.EventName {
	case VideoEventsPackage.VideoPublishedEventName, VideoEventsPackage.VideoDeletedEventName:
	default:
		return nil
	}
	if c == nil || c.projector == nil {
		return fmt.Errorf("feed projector cannot be nil")
	}
	if envelope.EventID == "" {
		return fmt.Errorf("feed event id cannot be empty")
	}
	delivery, ok := consumer.DeliveryFromContext(ctx)
	if !ok || delivery.Topic == "" || delivery.Partition < 0 || delivery.Offset < 0 {
		return fmt.Errorf("feed kafka delivery metadata is invalid")
	}

	var event VideoEventsPackage.VideoPublishedEvent
	if err := json.Unmarshal(envelope.Payload, &event); err != nil {
		return fmt.Errorf("decode feed event payload: %w", err)
	}
	if event.SubmissionID == "" {
		return fmt.Errorf("feed submission id cannot be empty")
	}
	if envelope.EventName == VideoEventsPackage.VideoDeletedEventName {
		if err := c.projector.Delete(ctx, delivery, envelope.EventID, event.SubmissionID); err != nil {
			return fmt.Errorf("delete feed projection: %w", err)
		}
		return nil
	}
	if envelope.Timestamp <= 0 {
		return fmt.Errorf("feed published timestamp must be positive")
	}
	if err := c.projector.Publish(ctx, delivery, envelope.EventID, event.SubmissionID, envelope.Timestamp); err != nil {
		return fmt.Errorf("publish feed projection: %w", err)
	}
	return nil
}
