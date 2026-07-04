package kafka

import (
	"MLC_GO/internal/consumer"
	HGKafkaPackage "MLC_GO/internal/pkg/kafka"
	"context"

	"github.com/twmb/franz-go/pkg/kgo"
)

// StartDomainEventConsumer 把统一 Kafka Consumer 和业务 Handler 连接起来。
func StartDomainEventConsumer(ctx context.Context, cli *kgo.Client, dlqTopic string, handler consumer.Handler) error {
	// HGNewBaseConsumer 负责 poll、提交 offset、失败进 DLQ 等 Kafka 通用能力。
	base := HGKafkaPackage.HGNewBaseConsumer(cli, dlqTopic)
	return base.HGStartConsume(ctx, func(ctx context.Context, record *kgo.Record) error {
		// 业务 Handler 只接收稳定 EventEnvelope，不直接依赖 kgo.Record。
		envelope, err := consumer.DecodeEnvelope(record.Value)
		if err != nil {
			return err
		}
		return handler.Handle(ctx, envelope)
	})
}
