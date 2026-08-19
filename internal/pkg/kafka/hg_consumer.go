/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-07-04 16:36:21
 * @LastEditors: Harley harelysoa@qq.com
 * @LastEditTime: 2026-08-19 16:02:51
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

// hgTerminalError 自定义错误包装结构体，内嵌 cause error，用来包裹原始真实错误。
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

// hgIsTerminalError 判断传入的 err 是否是终止型错误（terminal error）。
// 业务语义：遇到该类错误，代表流程不应该重试，直接终止当前逻辑；普通错误可以重试。
//	@param err 
//	@return bool 
func hgIsTerminalError(err error) bool {
	var terminal hgTerminalError

	// errors.As 要求传入目标指针，去匹配 error chain 里动态类型。
	// 但 hgTerminalError 没实现 error，不能直接 return hgTerminalError{cause: err} 
	// errors.As 遍历错误链（支持 Unwrap）
	// 尝试把链上某一层错误赋值到 &terminal
	// 如果错误链中存在类型为 hgTerminalError 的错误实例 → 返回 true，代表是终止错误；否则返回 false。
	return errors.As(err, &terminal)
}

// HGIsTerminalError reports whether a consumer error is a non-retryable protocol or data failure.
func HGIsTerminalError(err error) bool { return hgIsTerminalError(err) }

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

// HGBatchFailureIndex returns the failed record index carried by a batch error.
func HGBatchFailureIndex(err error, length int) int { return hgBatchFailureIndex(err, length) }

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
	// 消费循环，阻塞执行
	// consumeLoop 是阻塞循环，内部应该监听 ctx.Done()，ctx 取消后循环退出；
	// 当 consumeLoop 函数返回，代表消费循环结束；
	// return ctx.Err()：把上下文退出原因作为函数返回值。
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

// consumeBatchLoop 这是 HGBaseConsumer 的核心批量消费主循环，基于 kgo (kafka‑go)，实现批量拉取 Kafka 消息、批量业务处理、区分可重试 / 终止错误、DLQ 死信队列、进度上报 lag 观测、offset 提交、panic 保护、rebalance 重平衡配合的完整消费逻辑，是消费者的干活主循环
//	@param ctx 
//	@param handle 
func (b *HGBaseConsumer) consumeBatchLoop(ctx context.Context, handle HGRecordBatchHandler) {
	if b.lagObserver != nil {
		defer b.lagObserver.Close()
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			// 捕获业务handler panic，不直接让整个进程挂掉，打指标+堆栈日志
			hgKafkaConsumerPanics.Add(1)
			logHG.ErrFInfo("kafka batch consumer panic recovered err=%v stack=%s", recovered, string(debug.Stack()))
		}
	}()
	for {//拉取消息 → 处理 → 提交 offset，直到ctx取消

		// PollRecords 这里 Consumer 向 Kafka Broker 请求数据。 
		// fetches 本次 Poll 从 Kafka 拉回来的数据集合
		fetches := b.cli.PollRecords(ctx, hgConsumerMaxPollRecords)

		// if 如果ctx.Err()!=nil：上下文取消，调用AllowRebalance()允许重平衡，退出循环，消费者退出。
		if ctx.Err() != nil {
			b.cli.AllowRebalance()
			return
		}

		// if fetches.Errors() 这一次 Poll Kafka 的过程中是否发生了错误。 
		// 如果拉取过程 broker 返回错误（fetches.Errors()）：统计拉取错误指标，允许重平衡，continue 下一轮循环，不处理这批数据
		if errs := fetches.Errors(); len(errs) > 0 {
			hgKafkaFetchErrors.Add(uint64(len(errs)))

			// AllowRebalance 告知客户端本轮处理结束，可以参与 rebalance 分区再分配；如果不调用，会阻塞 rebalance。
			b.cli.AllowRebalance()
			continue
		}

		// b ObserveFetch 统计这次 Kafka Poll 拉取的数据情况，并根据每条 Record 的 offset / partition 等信息，计算或观测 Consumer Lag。
		// 把本次拉取到的消息交给 lag 观测器，内部就会调用前面你看到的hgAdvance，更新内存里消费进度，用来计算消费 lag 堆积指标
		b.lagObserver.ObserveFetch(fetches)
		startedAt := time.Now()
		var commitRecords []*kgo.Record

		// failedOffsets 嵌套map，用于存储每个 topic 下每个 partition 的失败 offset 信息。
		failedOffsets := make(map[string]map[int32]kgo.EpochOffset)
		for _, fetch := range fetches {
			for _, topic := range fetch.Topics {
				for _, partition := range topic.Partitions {
					if len(partition.Records) == 0 {
						continue
					}
					traceCtx := HGExtractTraceFromRecordContext(ctx, partition.Records[0])

					//  遍历每个拉取到的分区数据，调用业务 handler
					// kgo 的 fetches 结构：fetches → fetch → topic → partition → Records []
					// 按 partition 粒度批量调用业务 handler：一个分区的多条记录一次性丢给HGRecordBatchHandler业务回调
					err := hgInvokeRecordBatchHandler(traceCtx, handle, partition.Records)
					if err == nil {
						for _, record := range partition.Records {

							// 全部记录标记ObserveSuccessful更新 lag 进度
							b.lagObserver.ObserveSuccessful(record)
						}
						// 把该分区最后一条记录加入commitRecords，后续用来提交 offset。
						commitRecords = append(commitRecords, partition.Records[len(partition.Records)-1])
						continue
					}
					hgKafkaHandlerFailures.Add(1)

					// if erminal 终止错误：不可重试，消息进 DLQ 死信队列
					if hgIsTerminalError(err) {
						// 语义：坏消息丢去死信，跳过坏消息，继续消费后面的消息。
						hgKafkaTerminalFailures.Add(1)
					} else { // Retryable 可重试错误：不进 DLQ 或者 DLQ 发送失败
						// 将失败消息本身写入 failedOffsets，下一轮从头重新消费这条失败消息。
						hgKafkaRetryableFailures.Add(1)
					}
					// 获取这批里第几条记录是失败点，支持批量处理中部分成功部分失败。
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
						// 记录哪些分区需要重置到哪个 offset 重新消费。
						failedOffsets[failedRecord.Topic][failedRecord.Partition] = kgo.EpochOffset{Epoch: failedRecord.LeaderEpoch, Offset: failedRecord.Offset}
					}
				}
			}
		}
		// 收尾：指标统计 + 提交 offset + 重置消费位置；统计本次批量消费：消息数量、处理耗时指标。
		hgObserveConsumeBatch(fetches.NumRecords(), time.Since(startedAt))
		// 将commitRecords里成功的 offset 提交到 kafka
		// 根据failedOffsets，对失败的分区执行Seek 重置 offset，下一轮 poll 就会从这个失败 offset 重新拉取消息
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

// hgCommitAndRestore 提交成功 offset + 重置失败分区消费位点 + 放行重平衡。
//	@param ctx 
//	@param commitRecords 需要提交 offset 的消息；kgo 的CommitRecords语义：取每条 record 的 Offset+1 作为 commit offset，标记该消息以及之前全部已经消费完成。只需要传入分区最后一条成功记录，不需要传全部成功消息
//	@param failedOffsets topic → partition → EpochOffset(LeaderEpoch + Offset) 保存处理失败的分区需要跳回到哪个 offset 重新消费，来自上一步业务处理失败逻辑。
func (b *HGBaseConsumer) hgCommitAndRestore(ctx context.Context, 
	commitRecords []*kgo.Record, 
	failedOffsets map[string]map[int32]kgo.EpochOffset) {
	if len(commitRecords) > 0 {
		startedAt := time.Now()
		// CommitRecords 使用 kgo CommitRecords，向 Kafka broker 提交消费位点；
		// 埋点指标：提交条数、耗时、是否出错；
		// ⚠️注意：这里没有处理 Commit 返回的 err，没有重试、没有日志。
		// 即便 commit 失败，代码依然继续往下执行，不会阻断后续逻辑。Kafka offset commit 本身允许失败，下一轮消费还会再次提交。
		err := b.cli.CommitRecords(ctx, commitRecords...)
		hgObserveCommit(len(commitRecords), time.Since(startedAt), err)
	}
	if len(failedOffsets) > 0 {
		// SetOffsets 是 kgo 客户端内存 seek：修改本地客户端下一次 Poll 读取的起始 offset，不会立刻 commit 到 broker。
		b.cli.SetOffsets(failedOffsets)
	}
	// kgo 机制：消费处理期间默认阻止 rebalance，防止处理消息过程中被剥夺分区。
	// 每一轮消费全部做完（提交 + seek 都完成）调用AllowRebalance()，告诉客户端：本批次处理完毕，可以参与重平衡，允许分区被回收分配。
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
