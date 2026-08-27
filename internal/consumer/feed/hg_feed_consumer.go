/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-07-25 21:15:01
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-08-26 17:31:29
 * @FilePath: /MLC_GO/internal/consumer/feed/hg_feed_consumer.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
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

// 主要用于Kafka 消费者 `Consumer.Handle` 的处理入口，属于典型的 **CQRS + 事件驱动写模型**—— 消费 "视频事件"，通过 `projector`（投影器）把事件落成 feed（信息流）的物化视图。
//
//	@param ctx
//	@param envelope 收到一条 `events.EventEnvelope`（事件信封），根据事件类型做**路由 + 校验 + 投影落库**三件事：
// 	 - 只关心两个事件：`VideoPublished`（发布）和 `VideoDeleted`（删除）
// 	 - 其余事件直接忽略（`return nil`，正常返回、不视为错误）
// 	 - 分别调用 `projector.Publish` / `projector.Delete` 写投影
//	@return error
func (c *Consumer) Handle(ctx context.Context, envelope events.EventEnvelope) error {

	// switch 只处理发布 / 删除两类事件，其他事件（如审核中、转码完成等）**静默跳过**。这里 `return nil` 而不是报错 —— 因为对不关心的事件返回错误会导致 Kafka 重试 / 死信，白白浪费，跳过是正确语义。
	switch envelope.EventName {
	case VideoEventsPackage.VideoPublishedEventName, VideoEventsPackage.VideoDeletedEventName:
	default:
		return nil
	}
	if c == nil || c.projector == nil { // 消费者 / 投影器未初始化`
		return fmt.Errorf("feed projector cannot be nil")
	}
	if envelope.EventID == "" { // 事件 ID 缺失（幂等键），无法安全幂等落库
		return fmt.Errorf("feed event id cannot be empty")
	}
	delivery, ok := consumer.DeliveryFromContext(ctx)
	if !ok || delivery.Topic == "" || delivery.Partition < 0 || delivery.Offset < 0 { // Kafka 投递元数据非法
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
		// 删除事件不需要校验 timestamp（删稿不需要时间），直接调投影器的删除方法，然后**提前返回**
		if err := c.projector.Delete(ctx, delivery, envelope.EventID, event.SubmissionID); err != nil {
			return fmt.Errorf("delete feed projection: %w", err)
		}
		return nil
	}
	if envelope.Timestamp <= 0 {
		return fmt.Errorf("feed published timestamp must be positive")
	}
	// 校验通过后调用 `Publish`，把事件落成 feed 投影
	if err := c.projector.Publish(ctx, delivery, envelope.EventID, event.SubmissionID, envelope.Timestamp); err != nil {
		return fmt.Errorf("publish feed projection: %w", err)
	}
	return nil
}
