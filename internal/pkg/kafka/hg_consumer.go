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
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"MLC_GO/internal/pkg/logHG"

	"github.com/twmb/franz-go/pkg/kgo"
)

type hgTerminalError struct{ cause error }

func (e hgTerminalError) Error() string { return e.cause.Error() }
func (e hgTerminalError) Unwrap() error { return e.cause }

// HGNewTerminalError 标记无法通过重试恢复的协议或数据错误；成功写入 DLQ 后可推进源 offset。
func HGNewTerminalError(err error) error {
	if err == nil {
		return nil
	}
	return hgTerminalError{cause: err}
}

func hgIsTerminalError(err error) bool {
	var terminal hgTerminalError
	return errors.As(err, &terminal)
}

const hgConsumerMaxPollRecords = 500

// HGRecordHandler 是统一消费处理函数。
//
// handler 返回 nil 后才提交 offset；返回 error 时消息会进入 DLQ，失败分区从当前消息继续重试。
type HGRecordHandler func(ctx context.Context, record *kgo.Record) error

// HGRecordBatchHandler 处理同一 topic/partition 内保持 offset 顺序的有界记录批次。
type HGRecordBatchHandler func(context.Context, []*kgo.Record) error

type hgBatchRecordError struct {
	index int
	err   error
}

func (e hgBatchRecordError) Error() string { return e.err.Error() }
func (e hgBatchRecordError) Unwrap() error { return e.err }

// HGNewBatchRecordError 标记批次中具体失败记录，防止 DLQ 和 offset 错误跳过合法消息。
func HGNewBatchRecordError(index int, err error) error {
	if err == nil {
		return nil
	}
	return hgBatchRecordError{index: index, err: err}
}

func hgBatchFailureIndex(err error, length int) int {
	var batchErr hgBatchRecordError
	if errors.As(err, &batchErr) && batchErr.index >= 0 && batchErr.index < length {
		return batchErr.index
	}
	return 0
}

// HGBaseConsumer 是 franz-go 消费基类。
//
// 它封装手动提交 offset、panic 保护、DLQ 投递与 context 退出；具体业务消费者只需要实现 HGRecordHandler。
type HGBaseConsumer struct {
	cli         *kgo.Client
	dlqTopic    string
	lagObserver *HGConsumerLagObserver
	once        sync.Once
}

// HGNewBaseConsumer 创建统一消费基类。
func HGNewBaseConsumer(cli *kgo.Client, dlqTopic string) *HGBaseConsumer {
	return &HGBaseConsumer{cli: cli, dlqTopic: dlqTopic}
}

// HGNewBaseConsumerWithLagObserver creates a consumer with explicit group/topic lag identity.
func HGNewBaseConsumerWithLagObserver(cli *kgo.Client, dlqTopic string, group string, topics []string) *HGBaseConsumer {
	return HGNewBaseConsumerWithObserver(cli, dlqTopic, HGNewConsumerLagObserver(group, topics))
}

// HGNewBaseConsumerWithObserver creates a consumer using an observer registered during client setup.
func HGNewBaseConsumerWithObserver(cli *kgo.Client, dlqTopic string, observer *HGConsumerLagObserver) *HGBaseConsumer {
	return &HGBaseConsumer{
		cli:         cli,
		dlqTopic:    dlqTopic,
		lagObserver: observer,
	}
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

// HGRunConsume 同步运行消费循环，供应用 runtime 管理退出和等待。
func (b *HGBaseConsumer) HGRunConsume(ctx context.Context, handle HGRecordHandler) error {
	if b == nil || b.cli == nil {
		return fmt.Errorf("kafka consumer client cannot be nil")
	}
	if handle == nil {
		return fmt.Errorf("kafka consumer handler cannot be nil")
	}
	b.consumeLoop(ctx, handle)
	return ctx.Err()
}

// HGRunConsumeBatch 同步运行分区内批处理消费循环。
func (b *HGBaseConsumer) HGRunConsumeBatch(ctx context.Context, handle HGRecordBatchHandler) error {
	if b == nil || b.cli == nil {
		return fmt.Errorf("kafka consumer client cannot be nil")
	}
	if handle == nil {
		return fmt.Errorf("kafka consumer batch handler cannot be nil")
	}
	b.consumeBatchLoop(ctx, handle)
	return ctx.Err()
}

func (b *HGBaseConsumer) consumeBatchLoop(ctx context.Context, handle HGRecordBatchHandler) {
	if b.lagObserver != nil {
		defer b.lagObserver.Close()
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			hgKafkaConsumerPanics.Add(1)
			logHG.ErrFInfo("kafka batch consumer panic recovered err=%v stack=%s", recovered, string(debug.Stack()))
		}
	}()
	for {
		fetches := b.cli.PollRecords(ctx, hgConsumerMaxPollRecords)
		if ctx.Err() != nil {
			b.cli.AllowRebalance()
			return
		}
		if errs := fetches.Errors(); len(errs) > 0 {
			hgKafkaFetchErrors.Add(uint64(len(errs)))
			b.cli.AllowRebalance()
			continue
		}
		b.lagObserver.ObserveFetch(fetches)
		startedAt := time.Now()
		var commitRecords []*kgo.Record
		failedOffsets := make(map[string]map[int32]kgo.EpochOffset)
		for _, fetch := range fetches {
			for _, topic := range fetch.Topics {
				for _, partition := range topic.Partitions {
					if len(partition.Records) == 0 {
						continue
					}
					traceCtx := HGExtractTraceFromRecordContext(ctx, partition.Records[0])
					err := hgInvokeRecordBatchHandler(traceCtx, handle, partition.Records)
					if err == nil {
						for _, record := range partition.Records {
							b.lagObserver.ObserveSuccessful(record)
						}
						commitRecords = append(commitRecords, partition.Records[len(partition.Records)-1])
						continue
					}
					hgKafkaHandlerFailures.Add(1)
					if hgIsTerminalError(err) {
						hgKafkaTerminalFailures.Add(1)
					} else {
						hgKafkaRetryableFailures.Add(1)
					}
					failureIndex := hgBatchFailureIndex(err, len(partition.Records))
					failedRecord := partition.Records[failureIndex]
					hgKafkaDLQWrites.Add(1)
					if dlqErr := HGSendDLQ(traceCtx, failedRecord, b.dlqTopic, err.Error()); dlqErr == nil && hgIsTerminalError(err) {
						hgKafkaDLQSuccesses.Add(1)
						for _, record := range partition.Records[:failureIndex] {
							b.lagObserver.ObserveSuccessful(record)
						}
						b.lagObserver.ObserveTerminal(failedRecord)
						commitRecords = append(commitRecords, failedRecord)
						if failureIndex+1 < len(partition.Records) {
							next := partition.Records[failureIndex+1]
							if failedOffsets[next.Topic] == nil {
								failedOffsets[next.Topic] = make(map[int32]kgo.EpochOffset)
							}
							failedOffsets[next.Topic][next.Partition] = kgo.EpochOffset{Epoch: next.LeaderEpoch, Offset: next.Offset}
						}
					} else {
						for _, record := range partition.Records[:failureIndex] {
							b.lagObserver.ObserveSuccessful(record)
						}
						b.lagObserver.ObserveRetryable(failedRecord)
						if dlqErr != nil {
							hgKafkaDLQFailures.Add(1)
						}
						if failedOffsets[failedRecord.Topic] == nil {
							failedOffsets[failedRecord.Topic] = make(map[int32]kgo.EpochOffset)
						}
						failedOffsets[failedRecord.Topic][failedRecord.Partition] = kgo.EpochOffset{Epoch: failedRecord.LeaderEpoch, Offset: failedRecord.Offset}
					}
				}
			}
		}
		hgObserveConsumeBatch(fetches.NumRecords(), time.Since(startedAt))
		b.hgCommitAndRestore(ctx, commitRecords, failedOffsets)
	}
}

func hgInvokeRecordBatchHandler(ctx context.Context, handle HGRecordBatchHandler, records []*kgo.Record) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("kafka batch handler panic: %v", recovered)
		}
	}()
	return handle(ctx, records)
}

func (b *HGBaseConsumer) hgCommitAndRestore(ctx context.Context, commitRecords []*kgo.Record, failedOffsets map[string]map[int32]kgo.EpochOffset) {
	if len(commitRecords) > 0 {
		startedAt := time.Now()
		err := b.cli.CommitRecords(ctx, commitRecords...)
		hgObserveCommit(len(commitRecords), time.Since(startedAt), err)
	}
	if len(failedOffsets) > 0 {
		b.cli.SetOffsets(failedOffsets)
	}
	b.cli.AllowRebalance()
}

func (b *HGBaseConsumer) consumeLoop(ctx context.Context, handle HGRecordHandler) {
	if b.lagObserver != nil {
		defer b.lagObserver.Close()
	}
	defer func() {
		if r := recover(); r != nil {
			hgKafkaConsumerPanics.Add(1)
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
			hgKafkaFetchErrors.Add(uint64(len(errs)))
			logHG.ErrFInfo("kafka fetch error errs=%v", errs)
			b.cli.AllowRebalance()
			continue
		}
		b.lagObserver.ObserveFetch(fetches)

		batchStartedAt := time.Now()
		commitRecords, failedOffsets := hgProcessFetchBatchObserved(ctx, fetches, handle, b.lagObserver, func(traceCtx context.Context, record *kgo.Record, handleErr error) bool {
			hgKafkaHandlerFailures.Add(1)
			logHG.ErrFInfo("kafka consume handle failed topic=%s partition=%d offset=%d err=%v", record.Topic, record.Partition, record.Offset, handleErr)
			hgKafkaDLQWrites.Add(1)
			if dlqErr := HGSendDLQ(traceCtx, record, b.dlqTopic, handleErr.Error()); dlqErr != nil {
				hgKafkaDLQFailures.Add(1)
				logHG.ErrFInfo("kafka consume dlq failed topic=%s partition=%d offset=%d err=%v", record.Topic, record.Partition, record.Offset, dlqErr)
				return false
			}
			hgKafkaDLQSuccesses.Add(1)
			if hgIsTerminalError(handleErr) {
				hgKafkaTerminalFailures.Add(1)
			} else {
				hgKafkaRetryableFailures.Add(1)
			}
			return hgIsTerminalError(handleErr)
		})
		hgObserveConsumeBatch(fetches.NumRecords(), time.Since(batchStartedAt))

		if len(commitRecords) > 0 {
			// CommitRecords 支持变参批量提交；每个分区只传最后一条连续成功记录，单批只产生一次提交请求。
			commitStartedAt := time.Now()
			err := b.cli.CommitRecords(ctx, commitRecords...)
			hgObserveCommit(len(commitRecords), time.Since(commitStartedAt), err)
			if err != nil {
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
	onFailure func(context.Context, *kgo.Record, error) bool,
) ([]*kgo.Record, map[string]map[int32]kgo.EpochOffset) {
	return hgProcessFetchBatchObserved(ctx, fetches, handle, nil, onFailure)
}

func hgProcessFetchBatchObserved(
	ctx context.Context,
	fetches kgo.Fetches,
	handle HGRecordHandler,
	lagObserver *HGConsumerLagObserver,
	onFailure func(context.Context, *kgo.Record, error) bool,
) ([]*kgo.Record, map[string]map[int32]kgo.EpochOffset) {
	commitRecords := make([]*kgo.Record, 0, len(fetches))
	failedOffsets := make(map[string]map[int32]kgo.EpochOffset)

	for _, fetch := range fetches {
		for _, topic := range fetch.Topics {
			for _, partition := range topic.Partitions {
				var lastSucceeded *kgo.Record
				for _, record := range partition.Records {
					traceCtx := HGExtractTraceFromRecordContext(ctx, record)
					if err := hgInvokeRecordHandler(traceCtx, handle, record); err != nil {
						parked := false
						if onFailure != nil {
							parked = onFailure(traceCtx, record, err)
						}
						if parked {
							lagObserver.ObserveTerminal(record)
							lastSucceeded = record
							continue
						}
						lagObserver.ObserveRetryable(record)
						if failedOffsets[record.Topic] == nil {
							failedOffsets[record.Topic] = make(map[int32]kgo.EpochOffset)
						}
						failedOffsets[record.Topic][record.Partition] = kgo.EpochOffset{
							Epoch:  record.LeaderEpoch,
							Offset: record.Offset,
						}
						break
					}
					lagObserver.ObserveSuccessful(record)
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

func hgInvokeRecordHandler(ctx context.Context, handle HGRecordHandler, record *kgo.Record) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("kafka record handler panic: %v", recovered)
		}
	}()
	return handle(ctx, record)
}
