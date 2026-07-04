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

// Publish 发布领域事件到 Kafka。
func (b *KafkaEventBus) Publish(ctx context.Context, event events.DomainEvent) error {
	if event == nil {
		// 空事件视为无操作，避免调用方在可选事件场景额外分支处理。
		return nil
	}
	if b.topic == "" {
		return fmt.Errorf("event bus topic cannot be empty")
	}
	// EventKey 用于 Kafka 分区路由，保证同一业务实体的事件尽量落到同一分区。
	return HGKafkaPackage.HGSendBusinessEvent(ctx, b.topic, event.EventKey(), event)
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
