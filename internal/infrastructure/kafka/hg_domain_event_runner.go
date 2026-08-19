/*
 * @Author: Harley harelysoa@qq.com
 * @Date: 2026-08-15 14:27:04
 * @LastEditors: Harley harelysoa@qq.com
 * @LastEditTime: 2026-08-19 15:17:58
 * @FilePath: /MLC_GO/internal/infrastructure/kafka/hg_domain_event_runner.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
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
func RunDomainEventConsumerWithLagObserver(ctx context.Context,
	cli *kgo.Client, 
	dlqTopic string, 
	observer *HGKafkaPackage.HGConsumerLagObserver, 
	handler consumer.Handler) error {
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

// hgDomainEventRecordHandler 这是一个适配器（wrapper / 装饰器）函数：
// 把底层 kgo 的 *kgo.Record Kafka 原始消息，转换成上层业务层需要的 consumer.Envelope（事件信封），调用业务 handler.Handle()；同时做错误分类，版本不兼容错误标记为 Terminal 终止错误。
// 隔离：业务代码不直接依赖 kgo 库，业务只感知 EventEnvelope，底层 kafka 实现可以替换。
//	@param handler 业务实现的处理器
//	@return HGKafkaPackage.HGRecordHandler 框架层定义的单条消息处理函数类型
func hgDomainEventRecordHandler(handler consumer.Handler) HGKafkaPackage.HGRecordHandler {
	return func(ctx context.Context, record *kgo.Record) error {
		// 把这条消息的 kafka 元信息（topic、分区、offset）塞进 ctx。
		// 业务代码在 handler.Handle 内部可以从 ctx 取出 Delivery，获取这条消息的 kafka 位点信息，业务层不需要接触 kgo.Record 结构体。
		ctx = consumer.WithDelivery(ctx, consumer.Delivery{Topic: record.Topic, Partition: record.Partition, Offset: record.Offset})
		// record.Value 是 kafka 二进制 payload；
		// DecodeEnvelope：反序列化，解析成统一的事件信封 EventEnvelope。
		// Envelope 一般包含：事件版本、事件类型、业务 payload、元数据、traceId 等。		
		envelope, err := consumer.DecodeEnvelope(record.Value)
		if err != nil {
			// ErrUnsupportedEnvelopeVersion：消息协议版本不支持。
			// 比如老版本代码收到新版本格式事件，无法解析，这个错误不可重试，包装成 hgTerminalError（终止错误）。
			if errors.Is(err, consumer.ErrUnsupportedEnvelopeVersion) {
				return HGKafkaPackage.HGNewTerminalError(err)
			}
			// 其他解码错误（例如 json 解析失败、格式损坏）：直接返回普通 error，属于可重试错误，不会包装为 Terminal；上层会回滚 offset，不断重试这条坏消息。
			return err
		}
		// 把解析好的信封交给业务 handler 处理，业务返回的 error 直接透传给上层消费循环。
		return handler.Handle(ctx, envelope)
	}
}
