/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-07-04 16:36:21
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-07-23 16:36:15
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

const hgConsumerMaxPollRecords = 500

// HGRecordHandler 是统一消费处理函数。
//
// handler 返回 nil 后才提交 offset；返回 error 时消息会进入 DLQ，失败分区从当前消息继续重试。
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
// 注意：client 必须通过 ConsumeTopics、ConsumerGroup、DisableAutoCommit 和 BlockRebalanceOnPoll 配置消费组；
// 这里使用有界批次处理并批量提交 offset，不在请求路径创建 goroutine，不使用无界 channel。
func (b *HGBaseConsumer) HGStartConsume(ctx context.Context, handle HGRecordHandler) error {
	if b == nil || b.cli == nil {
		return fmt.Errorf("kafka consumer client cannot be nil")
	}
	if handle == nil {
		return fmt.Errorf("kafka consumer handler cannot be nil")
	}

	// 保证某段代码只执行一次，即使多个 goroutine 同时调用，也只会有一个执行成功
	b.once.Do(func() {
		// 消费循环是长生命周期 goroutine，生命周期由传入 ctx 控制。
		go b.consumeLoop(ctx, handle)
	})

	return nil
}

func (b *HGBaseConsumer) consumeLoop(ctx context.Context, handle HGRecordHandler) {
	defer func() {
		if r := recover(); r != nil {
			// 长生命周期消费 goroutine 不能因为单条异常退出进程，先记录堆栈交给监控告警。
			logHG.ErrFInfo("kafka consumer panic recovered err=%v stack=%s", r, string(debug.Stack()))
		}
	}()

	for {
		// select类似：多个 channel 等待，哪个 channel 有数据，执行哪个。
		select {
		// 退出消费循环
		//ctx里面有一个Done()，即channel。正常Done channel没有数据，处于阻塞状态。调用cancel()变成：ctx.Done()发送关闭信号
		case <-ctx.Done():
			return
		default:
		}

		// 限制单批记录数，避免大 fetch 在慢 Handler 下长期阻塞 rebalance 或占用过多内存。
		fetches := b.cli.PollRecords(ctx, hgConsumerMaxPollRecords)
		if ctx.Err() != nil {
			b.cli.AllowRebalance()
			return
		}

		if errs := fetches.Errors(); len(errs) > 0 {
			logHG.ErrFInfo("kafka fetch error errs=%v", errs)
			b.cli.AllowRebalance()
			continue
		}

		commitRecords, failedOffsets := hgProcessFetchBatch(ctx, fetches, handle, func(traceCtx context.Context, record *kgo.Record, handleErr error) {
			logHG.ErrFInfo("kafka consume handle failed topic=%s partition=%d offset=%d err=%v", record.Topic, record.Partition, record.Offset, handleErr)
			if dlqErr := HGSendDLQ(traceCtx, record, b.dlqTopic, handleErr.Error()); dlqErr != nil {
				logHG.ErrFInfo("kafka consume dlq failed topic=%s partition=%d offset=%d err=%v", record.Topic, record.Partition, record.Offset, dlqErr)
			}
		})

		if len(commitRecords) > 0 {
			// CommitRecords 支持变参批量提交；每个分区只传最后一条连续成功记录，单批只产生一次提交请求。
			if err := b.cli.CommitRecords(ctx, commitRecords...); err != nil {
				// 提交失败会导致消息后续重复消费，因此业务 Handler 必须按事件 ID / 业务 key 保证幂等。
				logHG.ErrFInfo("kafka commit batch failed records=%d err=%v", len(commitRecords), err)
			}
		}
		if len(failedOffsets) > 0 {
			// Poll 已推进本地位置；显式回退到失败消息，防止下一轮跳过失败分区的未处理记录。
			b.cli.SetOffsets(failedOffsets)
		}
		b.cli.AllowRebalance()
	}
}

func hgProcessFetchBatch(
	ctx context.Context,
	fetches kgo.Fetches,
	handle HGRecordHandler,
	onFailure func(context.Context, *kgo.Record, error),
) ([]*kgo.Record, map[string]map[int32]kgo.EpochOffset) {
	commitRecords := make([]*kgo.Record, 0, len(fetches))
	failedOffsets := make(map[string]map[int32]kgo.EpochOffset)

	for _, fetch := range fetches {
		for _, topic := range fetch.Topics {
			for _, partition := range topic.Partitions {
				var lastSucceeded *kgo.Record
				for _, record := range partition.Records {
					traceCtx := HGExtractTraceFromRecord(record)
					if err := handle(traceCtx, record); err != nil {
						if onFailure != nil {
							onFailure(traceCtx, record, err)
						}
						if failedOffsets[record.Topic] == nil {
							failedOffsets[record.Topic] = make(map[int32]kgo.EpochOffset)
						}
						failedOffsets[record.Topic][record.Partition] = kgo.EpochOffset{
							Epoch:  record.LeaderEpoch,
							Offset: record.Offset,
						}
						break
					}
					lastSucceeded = record
				}
				if lastSucceeded != nil {
					commitRecords = append(commitRecords, lastSucceeded)
				}
			}
		}
	}

	if len(failedOffsets) == 0 {
		failedOffsets = nil
	}
	return commitRecords, failedOffsets
}
