/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-07-04 16:36:21
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-07-04 17:40:56
 * @FilePath: /MLC_GO/internal/pkg/kafka/hg_consumer.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 * 统一消费基类、自动offset管理、DLQ
 */

package HGKafkaPackage

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"

	"MLC_GO/internal/pkg/logHG"

	"github.com/twmb/franz-go/pkg/kgo"
)

// HGRecordHandler 是统一消费处理函数。
//
// handler 返回 nil 后才提交 offset；返回 error 时消息会进入 DLQ，且不会提交当前 offset，便于后续重试。
type HGRecordHandler func(ctx context.Context, record *kgo.Record) error

// HGBaseConsumer 是 franz-go 消费基类。
//
// 它封装手动提交 offset、panic 保护、DLQ 投递与 context 退出；具体业务消费者只需要实现 HGRecordHandler。
type HGBaseConsumer struct {
	cli      *kgo.Client
	dlqTopic string
	once     sync.Once
}

// HGNewBaseConsumer 创建统一消费基类。
func HGNewBaseConsumer(cli *kgo.Client, dlqTopic string) *HGBaseConsumer {
	return &HGBaseConsumer{cli: cli, dlqTopic: dlqTopic}
}

// HGStartConsume 启动消费循环，支持多topic、自动负载均衡
//
// 注意：franz-go 的订阅 topic/group 应尽量通过创建 client 时的 ConsumeTopics/ConsumerGroup 配置完成；
// 为保持调用简单，这里只负责 PollFetches 主循环，不在请求路径创建 goroutine，不使用无界 channel。
func (b *HGBaseConsumer) HGStartConsume(ctx context.Context, handle HGRecordHandler) error {
	if b == nil || b.cli == nil {
		return fmt.Errorf("kafka consumer client cannot be nil")
	}
	if handle == nil {
		return fmt.Errorf("kafka consumer handler cannot be nil")
	}

	b.once.Do(func() {
		go b.consumeLoop(ctx, handle)
	})

	return nil
}

func (b *HGBaseConsumer) consumeLoop(ctx context.Context, handle HGRecordHandler) {
	defer func() {
		if r := recover(); r != nil {
			logHG.ErrFInfo("kafka consumer panic recovered err=%v stack=%s", r, string(debug.Stack()))
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		fetches := b.cli.PollFetches(ctx)
		if ctx.Err() != nil {
			return
		}

		if errs := fetches.Errors(); len(errs) > 0 {
			logHG.ErrFInfo("kafka fetch error errs=%v", errs)
			continue
		}
		// 遍历所有拉取到的消息
		fetches.EachRecord(func(record *kgo.Record) {
			traceCtx := HGExtractTraceFromRecord(record)
			if err := handle(traceCtx, record); err != nil {
				logHG.ErrFInfo("kafka consume handle failed topic=%s partition=%d offset=%d err=%v", record.Topic, record.Partition, record.Offset, err)

				if dlqErr := HGSendDLQ(traceCtx, record, b.dlqTopic, err.Error()); dlqErr != nil {
					logHG.ErrFInfo("kafka consume dlq failed topic=%s partition=%d offset=%d err=%v", record.Topic, record.Partition, record.Offset, dlqErr)
				}
				// 失败不提交offset，下次重新消费
				return
			}
			// 业务处理成功，手动提交offset
			if err := b.cli.CommitRecords(ctx, record); err != nil {
				logHG.ErrFInfo("kafka commit record failed topic=%s partition=%d offset=%d err=%v", record.Topic, record.Partition, record.Offset, err)
			}
		})
	}
}
