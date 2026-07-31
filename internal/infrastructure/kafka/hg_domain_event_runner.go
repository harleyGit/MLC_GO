package kafka

import (
	"MLC_GO/internal/consumer"
	HGKafkaPackage "MLC_GO/internal/pkg/kafka"
	"context"
	"errors"
	"fmt"

	"github.com/twmb/franz-go/pkg/kgo"
)

// StartDomainEventConsumer 把统一 Kafka Consumer 和业务 Handler 连接起来。
func StartDomainEventConsumer(ctx context.Context, cli *kgo.Client, dlqTopic string, handler consumer.Handler) error {
	if handler == nil {
		return fmt.Errorf("domain event handler cannot be nil")
	}
	base := HGKafkaPackage.HGNewBaseConsumer(cli, dlqTopic)
	return base.HGStartConsume(ctx, hgDomainEventRecordHandler(handler))
}

// RunDomainEventConsumer 同步运行领域事件消费者，退出由 ctx 控制。
func RunDomainEventConsumer(ctx context.Context, cli *kgo.Client, dlqTopic string, handler consumer.Handler) error {
	return RunDomainEventConsumerObserved(ctx, cli, dlqTopic, "", nil, handler)
}

// RunDomainEventConsumerObserved synchronously runs a consumer with explicit lag metric identity.
func RunDomainEventConsumerObserved(ctx context.Context, cli *kgo.Client, dlqTopic string, group string, topics []string, handler consumer.Handler) error {
	return RunDomainEventConsumerWithLagObserver(ctx, cli, dlqTopic, HGKafkaPackage.HGNewConsumerLagObserver(group, topics), handler)
}

// RunDomainEventConsumerWithLagObserver runs a consumer with a pre-registered lag observer.
func RunDomainEventConsumerWithLagObserver(ctx context.Context, cli *kgo.Client, dlqTopic string, observer *HGKafkaPackage.HGConsumerLagObserver, handler consumer.Handler) error {
	if handler == nil {
		observer.Close()
		return fmt.Errorf("domain event handler cannot be nil")
	}
	// HGNewBaseConsumer 负责 poll、提交 offset、失败进 DLQ 等 Kafka 通用能力。
	base := HGKafkaPackage.HGNewBaseConsumerWithObserver(cli, dlqTopic, observer)
	if batchHandler, ok := handler.(consumer.BatchHandler); ok {
		return base.HGRunConsumeBatch(ctx, hgDomainEventBatchHandler(batchHandler))
	}
	return base.HGRunConsume(ctx, hgDomainEventRecordHandler(handler))
}

func hgDomainEventBatchHandler(handler consumer.BatchHandler) HGKafkaPackage.HGRecordBatchHandler {
	return func(ctx context.Context, records []*kgo.Record) error {
		delivered := make([]consumer.DeliveredEnvelope, 0, len(records))
		for index, record := range records {
			envelope, err := consumer.DecodeEnvelope(record.Value)
			if err != nil {
				if len(delivered) > 0 {
					if handleErr := handler.HandleBatch(ctx, delivered); handleErr != nil {
						return handleErr
					}
				}
				return HGKafkaPackage.HGNewBatchRecordError(index, HGKafkaPackage.HGNewTerminalError(err))
			}
			delivered = append(delivered, consumer.DeliveredEnvelope{
				Delivery: consumer.Delivery{Topic: record.Topic, Partition: record.Partition, Offset: record.Offset},
				Envelope: envelope,
			})
		}
		return handler.HandleBatch(ctx, delivered)
	}
}

func hgDomainEventRecordHandler(handler consumer.Handler) HGKafkaPackage.HGRecordHandler {
	return func(ctx context.Context, record *kgo.Record) error {
		ctx = consumer.WithDelivery(ctx, consumer.Delivery{Topic: record.Topic, Partition: record.Partition, Offset: record.Offset})
		// 业务 Handler 只接收稳定 EventEnvelope，不直接依赖 kgo.Record。
		envelope, err := consumer.DecodeEnvelope(record.Value)
		if err != nil {
			if errors.Is(err, consumer.ErrUnsupportedEnvelopeVersion) {
				return HGKafkaPackage.HGNewTerminalError(err)
			}
			return err
		}
		return handler.Handle(ctx, envelope)
	}
}
