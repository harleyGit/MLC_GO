/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-07-04 17:55:28
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-08-27 11:34:59
 * @FilePath: /MLC_GO/internal/infrastructure/eventbus/hg_event_bus.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package eventbus

import (
	"MLC_GO/internal/events"
	"MLC_GO/internal/outbox"
	HGKafkaPackage "MLC_GO/internal/pkg/kafka"
	"context"
	"fmt"
)

// EventBus 封装领域事件发布能力。
// 业务 service 只调用 Publish，不直接依赖 Kafka producer、topic 或序列化细节。
type EventBus interface {
	// Publish 发布一个领域事件；实现方决定直接发 Kafka 还是先写 Outbox。
	Publish(ctx context.Context, event events.DomainEvent) error
}

// KafkaEventBus 使用 Kafka 投递领域事件。
type KafkaEventBus struct {
	topic string
}

// NewKafkaEventBus 创建 Kafka 领域事件总线。
func NewKafkaEventBus(topic string) *KafkaEventBus {
	return &KafkaEventBus{topic: topic}
}


/** 发送领域事件的完整链路解说：
 业务代码 → `KafkaEventBus.Publish()` → 构建 `EventEnvelope` 信封 → json 序列化 → 组装 kafka record → 同步发送 kafka → kafka 消息 → Consumer 消费处理（统计、ClickHouse 落库、Redis 投影）
 
 - 核心设计：所有 kafka 领域消息统一使用 EventEnvelope 协议格式，消费端不管消息来自直接发送，还是 outbox 发出来的，格式完全一致，消费代码不用改。注释特意强调：`直接 Kafka 与 Outbox 必须发送同一种 Envelope 协议，否则消费者无法统一校验 EventID 和做幂等。`
*/
// Publish 是Kafka 事件总线（EventBus）发送侧代码，属于领域事件发布组件。
// 业务产出领域事件 `DomainEvent`，经过封装成统一的 `EventEnvelope`（事件信封协议），同步发送到 Kafka；前面你看过的 `Consumer.Handle()` 就是这套消息的消费端。
//
//	@param ctx
//	@param event
//	@return error
func (b *KafkaEventBus) Publish(ctx context.Context, event events.DomainEvent) error {
	if event == nil {
		// 空事件视为无操作，避免调用方在可选事件场景额外分支处理。
		return nil
	}
	if b.topic == "" {
		return fmt.Errorf("event bus topic cannot be empty")
	}
	// 把业务领域事件，包装成统一信封 `EventEnvelope`
	envelope, err := newKafkaEnvelope(event)
	if err != nil {
		return err
	}

	// 系统存在两种发消息路径：
	//  - 1）直接调用 Publish 直发 kafka；
	//  - 2）Outbox 模式：先写数据库 outbox 表，后台异步读表再发 kafka；
	// 两种路径产出的消息必须是一模一样的 Envelope 结构。消费端只解析 Envelope，拿`EventID`做幂等。如果格式不一样，消费端幂等逻辑直接失效。
	// 
	// 直接 Kafka 与 Outbox 必须发送同一种 Envelope 协议，否则消费者无法统一校验 EventID 和做幂等。
	// 调用底层发送函数，把信封发送到 kafka。
	// `envelope.EventKey`：kafka 消息 key，kafka 按 key 做分区哈希，相同 key 的消息落到同一个 partition，保证顺序。
	return HGKafkaPackage.HGSendBusinessEvent(ctx, b.topic, envelope.EventKey, envelope)
}

// newKafkaEnvelope 薄薄一层包装，调用公共包的 `NewEnvelope`，做错误包装加一层上下文，方便日志定位是构建 kafka 信封出错 
//	@param event 
//	@return events.EventEnvelope 
//	@return error 
func newKafkaEnvelope(event events.DomainEvent) (events.EventEnvelope, error) {
	envelope, err := events.NewEnvelope(event)
	if err != nil {
		return events.EventEnvelope{}, fmt.Errorf("build kafka event envelope: %w", err)
	}
	return envelope, nil
}

// OutboxEventBus 只把事件写入 MySQL Outbox，不在接口请求内直连 Kafka。
// 适用于发布视频、创建订单这类“数据库成功必须最终发出事件”的核心链路。
type OutboxEventBus struct {
	repo *outbox.Repository
}

// NewOutboxEventBus 创建基于 MySQL Outbox 的领域事件总线。
func NewOutboxEventBus(repo *outbox.Repository) *OutboxEventBus {
	return &OutboxEventBus{repo: repo}
}

// Publish 将领域事件写入 Outbox，等待 dispatcher 异步投递。
func (b *OutboxEventBus) Publish(ctx context.Context, event events.DomainEvent) error {
	if event == nil {
		// 空事件不落库，保持和 KafkaEventBus 一致的幂等空操作语义。
		return nil
	}
	if b == nil || b.repo == nil {
		return fmt.Errorf("outbox event bus repository cannot be nil")
	}
	// Save 只负责本地持久化，避免接口请求路径直接依赖 Kafka 可用性。
	return b.repo.Save(ctx, event)
}

// KafkaByteProducer 是 Outbox dispatcher 使用的 Kafka producer 适配器。
type KafkaByteProducer struct{}

// Send 投递 Outbox 中已经序列化好的事件字节。
func (KafkaByteProducer) Send(ctx context.Context, topic string, key string, payload []byte) error {
	return HGKafkaPackage.HGSendBusinessEventBytes(ctx, topic, key, payload)
}
