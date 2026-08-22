/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-07-04 16:36:21
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-08-22 17:43:19
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
//
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
//
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
	for { //拉取消息 → 处理 → 提交 offset，直到ctx取消

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
//
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

// consumeLoop 这是一个长生命周期 Kafka 消费协程的主循环，核心设计目标是：单条消息异常不拖垮整个消费进程、失败消息可追溯（DLQ）、可重试消息不丢、提交粒度可控、rebalance 不被慢 Handler 长时间阻塞。
//
//	@param ctx
//	@param handle
func (b *HGBaseConsumer) consumeLoop(ctx context.Context, handle HGRecordHandler) {
	/** lagObserver 是消费延迟（lag）观测器，负责统计各分区消费进度与最新 offset 的差距。
	用 defer 保证循环退出时释放观测资源（比如停止后台采样 goroutine、刷新 metrics）。
	显式判 nil，说明该观测器是可选组件。
	*/
	if b.lagObserver != nil {
		defer b.lagObserver.Close()
	}
	// Panic 兜底：长生命周期 goroutine 的保命机制
	// 设计意图：consumeLoop 通常跑在一个独立 goroutine 里，一旦 handle 业务逻辑或底层库触发 panic 且未被捕获，整个 goroutine 会直接退出，消费就停了 —— 而主进程可能根本感知不到。
	defer func() {
		/** 这里的策略是：
		recover() 捕获 panic；
		打点 hgKafkaConsumerPanics 供监控告警；
		打印完整堆栈 debug.Stack() 便于定位；
		不退出进程、不退出循环，defer 结束后函数返回（但注意：这里 recover 后函数会正常 return，循环不会自动继续 —— 需要外层有重启机制，或者这个 consumeLoop 被外层 for 循环重新调用）。

		潜在风险：如果某条消息必然触发 panic，且 offset 又被回退（见后文 SetOffsets），会形成 "panic → recover → 回退 → 再拉同一条 → 再 panic" 的死循环。通常配合 DLQ + terminal error 判定来打破。
		*/
		if r := recover(); r != nil {
			hgKafkaConsumerPanics.Add(1)
			// 长生命周期消费 goroutine 不能因为单条异常退出进程，先记录堆栈交给监控告警。
			logHG.ErrFInfo("kafka consumer panic recovered err=%v stack=%s", r, string(debug.Stack()))
		}
	}()

	/** 每轮循环开始先非阻塞地检查 ctx.Done()：
	外部调用 cancel() 后，ctx.Done() 关闭，case 命中，return 退出循环；
	没取消就走 default，继续往下拉取消息。

	为什么不用 select { case <-ctx.Done(): return; case fetches := <-... }？
	因为 PollRecords 是一个阻塞式调用（内部封装了 poll 等待），不是原生 channel，无法直接放进 select。所以采用 "循环开始检查 + Poll 内部也传 ctx" 的双重退出保障。
	*/
	for {
		// select类似：多个 channel 等待，哪个 channel 有数据，执行哪个。
		select {
		// 退出消费循环
		//ctx里面有一个Done()，即channel。正常Done channel没有数据，处于阻塞状态。调用cancel()变成：ctx.Done()发送关闭信号
		case <-ctx.Done():
			return
		default:
		}

		// 批量拉取消息；限制单批记录数，避免大 fetch 在慢 Handler 下长期阻塞 rebalance 或占用过多内存。
		fetches := b.cli.PollRecords(ctx, hgConsumerMaxPollRecords)
		// if 拉取后再次检查 ctx.Err()：如果在 Poll 阻塞期间 ctx 被取消，立即退出。
		if ctx.Err() != nil {
			// 通知底层客户端 "现在可以安全地执行 rebalance 了"。这是一个关键控制 —— 后面会看到，整个处理批次期间 rebalance 是被抑制的，只有处理完才放开。
			b.cli.AllowRebalance()
			return
		}

		// 返回本次拉取中各分区的错误（如网络抖动、offset 越界等）。
		if errs := fetches.Errors(); len(errs) > 0 {
			// 打点 + 日志后，不退出，continue 进入下一轮重试。
			hgKafkaFetchErrors.Add(uint64(len(errs)))
			logHG.ErrFInfo("kafka fetch error errs=%v", errs)
			// 放开 rebalance，因为本轮没有正常处理消息，不需要持有分区所有权。
			b.cli.AllowRebalance()
			continue
		}
		// 把本次拉取到的消息（含各分区最新 offset）喂给 lag 观测器，用于计算 "生产最新 offset - 当前消费 offset" 的延迟
		b.lagObserver.ObserveFetch(fetches)

		// 批量处理 + 失败分流
		batchStartedAt := time.Now()
		commitRecords, failedOffsets := hgProcessFetchBatchObserved(ctx, fetches, handle, b.lagObserver, func(traceCtx context.Context, record *kgo.Record, handleErr error) bool {
			// 失败计数与日志
			hgKafkaHandlerFailures.Add(1)
			// 批量处理失败，失败消息写入 DLQ，是一条日志
			logHG.ErrFInfo("kafka consume handle failed topic=%s partition=%d offset=%d err=%v", record.Topic, record.Partition, record.Offset, handleErr)
			// 写入 DLQ（死信队列）
			hgKafkaDLQWrites.Add(1)
			// 所有处理失败的消息都尝试写入 b.dlqTopic（死信主题），保留原始消息 + 错误原因，便于人工排查和重放。
			if dlqErr := HGSendDLQ(traceCtx, record, b.dlqTopic, handleErr.Error()); dlqErr != nil { // DLQ 写入失败时 return false—— 这个返回值的含义见下
				hgKafkaDLQFailures.Add(1)
				logHG.ErrFInfo("kafka consume dlq failed topic=%s partition=%d offset=%d err=%v", record.Topic, record.Partition, record.Offset, dlqErr)
				return false
			}
			//  错误分类：terminal vs retryable
			hgKafkaDLQSuccesses.Add(1)
			if hgIsTerminalError(handleErr) {
				/** terminal error（终态错误）：如消息格式非法、业务校验不通过、反序列化失败等。这类消息重试多少次都会失败，所以：
				- 已写入 DLQ 留档；
				- return true → 不加入 failedOffsets，offset 正常推进，不再重试。
				*/
				hgKafkaTerminalFailures.Add(1)
			} else {
				/** 如下游服务超时、数据库连接失败、网络抖动等。这类消息稍后重试可能成功，所以：
				也写入 DLQ（留档）；
				 - return false → 加入 failedOffsets，offset 回退，下一轮重新拉取。
				 - 回调返回值的语义推断：true = "该失败已妥善处理（进 DLQ），视为可提交，无需回退"；false = "需要回退 offset 重试"。
				*/
				hgKafkaRetryableFailures.Add(1)
			}
			return hgIsTerminalError(handleErr)
		})
		hgObserveConsumeBatch(fetches.NumRecords(), time.Since(batchStartedAt))

		// 批量提交 Offset
		if len(commitRecords) > 0 {
			// CommitRecords 支持变参批量提交；每个分区只传最后一条连续成功记录，单批只产生一次提交请求。
			commitStartedAt := time.Now()
			// 变参批量提交，一批只产生一次提交请求，降低 broker 压力；
			// 已经过压缩：每个分区只传最后一条连续成功记录（Kafka 提交 offset 的语义是 "提交此 offset 之前的都已处理"）
			err := b.cli.CommitRecords(ctx, commitRecords...)
			hgObserveCommit(len(commitRecords), time.Since(commitStartedAt), err)
			if err != nil { // 提交失败只打日志不重试，注释明确说明：
				// 提交失败会导致消息后续重复消费，因此业务 Handler 必须按事件 ID / 业务 key 保证幂等。

				// 提交失败会导致消息后续重复消费，因此业务 Handler 必须按事件 ID / 业务 key 保证幂等。
				logHG.ErrFInfo("kafka commit batch failed records=%d err=%v", len(commitRecords), err)
			}
		}
		// 回退失败 Offset
		if len(failedOffsets) > 0 {
			/** PollRecords 拉取后，底层客户端的本地消费位置已经向前推进了。
			对于 retryable 失败的消息，必须显式把 offset 回退到失败位置，否则下一轮 Poll 会跳过这些未处理成功的消息，造成消息丢失。
			SetOffsets 只影响下一次 Poll 的起始位置，不影响已提交的 offset
			*/
			// Poll 已推进本地位置；显式回退到失败消息，防止下一轮跳过失败分区的未处理记录。
			b.cli.SetOffsets(failedOffsets)
		}
		/** 每轮循环末尾统一放开 rebalance
		为什么要抑制 rebalance？
		如果在 Handler 处理到一半时发生 rebalance，当前分区被分配给其他 consumer，而本地还在处理这些消息，会导致：
		 - 重复消费（其他 consumer 也拉到了同样的消息）；
		 - offset 提交冲突；
		  -资源竞争。
		所以尽量在 "批次边界" 放开 rebalance，保证处理的原子性。
		*/
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

// hgProcessFetchBatchObserved  这是一个基于 kgo（segmentio/kafka-go 的替代库，franz-go）的批量消费处理函数，核心目标是：拉取一批消息后逐分区处理，同时通过 lagObserver 做可观测性埋点，并区分可跳过失败与可重试失败两种错误处理策略。
//
//	@param ctx
//	@param fetches 一次 PollFetches 拉到的全部消息，结构是 Fetch → Topic → Partition → Records
//	@param handle 单条消息的业务处理器
//	@param lagObserver 消费延迟 / 处理状态观察者，对每条消息打不同状态的指标
//	@param onFailure 单条消息处理失败时的回调
//	@return []*kgo.Record 每个分区最后一条成功（含 parked）处理的记录，用于后续提交 offset
//	@return map[string]map[int32]kgo.EpochOffset 需要重试的分区失败位点，topic → partition → EpochOffset，用于 seek 回退
func hgProcessFetchBatchObserved(
	ctx context.Context,
	fetches kgo.Fetches,
	handle HGRecordHandler,
	lagObserver *HGConsumerLagObserver,
	onFailure func(context.Context, *kgo.Record, error) bool,
) ([]*kgo.Record, map[string]map[int32]kgo.EpochOffset) {
	commitRecords := make([]*kgo.Record, 0, len(fetches))
	failedOffsets := make(map[string]map[int32]kgo.EpochOffset)

	/** 核心数据结构遍历
	 */
	for _, fetch := range fetches {
		for _, topic := range fetch.Topics {
			for _, partition := range topic.Partitions {
				/** lastSucceeded 按分区维护。
				每个分区处理完后，只把该分区最后一条成功记录加入 commitRecords。这是 Kafka 消费的标准做法 —— 提交一个分区的 offset 时，只需要提交该分区最大已处理 offset（实际提交时是 offset+1，由调用方完成），不需要逐条提交。
				*/
				var lastSucceeded *kgo.Record
				for _, record := range partition.Records {
					traceCtx := HGExtractTraceFromRecordContext(ctx, record)
					if err := hgInvokeRecordHandler(traceCtx, handle, record); err != nil {
						parked := false
						if onFailure != nil {
							parked = onFailure(traceCtx, record, err)
						}
						// 语义：这条消息业务上判定为 "不可恢复但可以跳过"（例如毒消息、格式错误、业务校验失败且已进死信队列），onFailure 内部通常已经做了落库 / 告警 / 转死信等操作，所以可以安全跳过，offset 正常推进。
						if parked {
							// 标记为终端状态（最终失败但已被处理掉，不会再重试）
							lagObserver.ObserveTerminal(record)
							// 关键：把这条失败消息也当作 "成功位点"，意味着它的 offset 会被提交
							lastSucceeded = record
							continue
						}

						/** 标记为可重试失败
						* 语义：这条消息遇到了可重试错误（如下游服务超时、数据库连接失败），需要：
						停止当前分区后续处理（保证顺序性，不能跳过失败消息处理后面的）

						记录失败位点，调用方后续会用 failedOffsets 对该分区执行 seek 回退到这个 offset，下次 poll 重新消费
						*/
						lagObserver.ObserveRetryable(record)
						if failedOffsets[record.Topic] == nil {
							// 保存该 topic/partition 的 LeaderEpoch 和 Offset，用于下一轮消费时回退到该位置重新拉取消息。
							failedOffsets[record.Topic] = make(map[int32]kgo.EpochOffset)
						}

						// LeaderEpoch 是 Kafka 为了解决脑裂 / Leader 切换后 offset 歧义引入的。seek 时同时带上 epoch 和 offset，kgo 可以校验该 offset 是否真的属于当前 Leader 的纪元，避免在 Leader 切换后 seek 到错误的消息。这是幂等 / 事务消费的标准做法。
						failedOffsets[record.Topic][record.Partition] = kgo.EpochOffset{
							Epoch:  record.LeaderEpoch,
							Offset: record.Offset,
						}
						/** 为什么 break 而不是 continue？
						Kafka 分区内消息是有序的。如果第 N 条处理失败且需要重试，第 N+1 条即使处理成功也不能提交，因为一旦提交了 N+1 的 offset，第 N 条就永远丢失了（至少一次语义下不允许）。所以遇到 retryable 错误必须立即 break，保留 lastSucceeded 为 N-1，后续只提交到 N-1。
						*/
						break
					}
					// 标记该消息处理成功，lagObserver 通常会记录处理延迟（now - record.Timestamp）
					lagObserver.ObserveSuccessful(record)
					// lastSucceeded 更新当前分区的最后成功位点
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
	// 如果没有任何失败，把 failedOffsets 置为 nil（而非空 map），方便调用方用 if failedOffsets != nil 判断是否需要 seek 回退。
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
