package HGKafkaPackage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

var (
	hgKafkaProduceRecords         atomic.Uint64
	hgKafkaProduceFailures        atomic.Uint64
	hgKafkaWrittenBatches         atomic.Uint64
	hgKafkaConsumeBatches         atomic.Uint64
	hgKafkaConsumeBatchRecords    atomic.Uint64
	hgKafkaConsumeBatchNanos      atomic.Uint64
	hgKafkaFetchErrors            atomic.Uint64
	hgKafkaHandlerFailures        atomic.Uint64
	hgKafkaDLQWrites              atomic.Uint64
	hgKafkaDLQFailures            atomic.Uint64
	hgKafkaDLQSuccesses           atomic.Uint64
	hgKafkaRetryableFailures      atomic.Uint64
	hgKafkaTerminalFailures       atomic.Uint64
	hgKafkaConsumerLagSequence    atomic.Uint64
	hgKafkaConsumerLagObservers   sync.Map
	hgKafkaCommits                atomic.Uint64
	hgKafkaCommitFailures         atomic.Uint64
	hgKafkaCommitPartitions       atomic.Uint64
	hgKafkaCommitNanos            atomic.Uint64
	hgKafkaConsumerPanics         atomic.Uint64
	hgKafkaGroupErrors            atomic.Uint64
	hgKafkaPartitionsAssigned     atomic.Uint64
	hgKafkaPartitionsRevoked      atomic.Uint64
	hgKafkaPartitionsLost         atomic.Uint64
	hgKafkaAssignedPartitionGauge atomic.Int64
)

// HGMetricHook 收集 Kafka Producer 和 Consumer 的低开销进程级指标。
type HGMetricHook struct{}

type hgConsumerLagPartition struct {
	highWatermark int64
	nextOffset    int64
	initialized   bool
}

// HGConsumerLagObserver 跟踪一个 Consumer 实例所拥有 partition 的应用处理位置。
//
// 内部保留 partition 粒度是为了在 rebalance 时精确清理，但 Prometheus 只导出 group/topic 聚合，
// 避免 partition、client_id 等标签放大时间序列。该指标表示“已观察到的应用处理 lag”，不是通过
// Kafka Admin API 查询的严格 committed-offset lag；commit 失败由独立 counter 和告警监控。
type HGConsumerLagObserver struct {
	id         uint64
	group      string
	mu         sync.RWMutex
	partitions map[string]map[int32]hgConsumerLagPartition
	closeOnce  sync.Once
}

// HGNewConsumerLagObserver 为明确的消费组和 topic 集合注册 observer，并提前暴露零值序列。
func HGNewConsumerLagObserver(group string, topics []string) *HGConsumerLagObserver {
	observer := &HGConsumerLagObserver{
		id:         hgKafkaConsumerLagSequence.Add(1),
		group:      group,
		partitions: make(map[string]map[int32]hgConsumerLagPartition, len(topics)),
	}
	for _, topic := range topics {
		if topic != "" && observer.partitions[topic] == nil {
			observer.partitions[topic] = make(map[int32]hgConsumerLagPartition)
		}
	}
	hgKafkaConsumerLagObservers.Store(observer.id, observer)
	return observer
}

// ObserveFetch 只更新 broker high watermark，不把已拉取但尚未成功处理的记录误算为完成。
func (o *HGConsumerLagObserver) ObserveFetch(fetches kgo.Fetches) {
	if o == nil {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, fetch := range fetches {
		for _, topic := range fetch.Topics {
			partitions := o.partitions[topic.Topic]
			if partitions == nil {
				partitions = make(map[int32]hgConsumerLagPartition)
				o.partitions[topic.Topic] = partitions
			}
			for _, partition := range topic.Partitions {
				state := partitions[partition.Partition]
				state.highWatermark = partition.HighWatermark
				if !state.initialized && len(partition.Records) > 0 {
					state.nextOffset = partition.Records[0].Offset
					state.initialized = true
				}
				partitions[partition.Partition] = state
			}
		}
	}
}

// ObserveSuccessful 在业务 Handler 成功后推进应用处理位置。
func (o *HGConsumerLagObserver) ObserveSuccessful(record *kgo.Record) {
	o.hgAdvance(record)
}

// ObserveRetryable 保留失败记录为未完成，下一轮从该 offset 重试时 lag 仍包含它。
func (o *HGConsumerLagObserver) ObserveRetryable(_ *kgo.Record) {}

// ObserveTerminal 仅在终止错误已可靠写入 DLQ 后推进位置。
func (o *HGConsumerLagObserver) ObserveTerminal(record *kgo.Record) {
	o.hgAdvance(record)
}

// ObservePartitionsRevoked 删除当前实例已不再拥有的 partition，防止 rebalance 后残留陈旧 lag。
func (o *HGConsumerLagObserver) ObservePartitionsRevoked(partitions map[string][]int32) {
	if o == nil {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	for topic, partitionIDs := range partitions {
		states := o.partitions[topic]
		for _, partitionID := range partitionIDs {
			delete(states, partitionID)
		}
	}
}

func (o *HGConsumerLagObserver) hgAdvance(record *kgo.Record) {
	if o == nil || record == nil {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	partitions := o.partitions[record.Topic]
	if partitions == nil {
		partitions = make(map[int32]hgConsumerLagPartition)
		o.partitions[record.Topic] = partitions
	}
	state := partitions[record.Partition]
	nextOffset := record.Offset + 1
	if !state.initialized || nextOffset > state.nextOffset {
		state.nextOffset = nextOffset
		state.initialized = true
	}
	partitions[record.Partition] = state
}

// Close 注销该 Consumer 实例的全部 lag 状态；Runtime 关闭和 client 创建失败路径都必须调用。
func (o *HGConsumerLagObserver) Close() {
	if o == nil {
		return
	}
	o.closeOnce.Do(func() {
		hgKafkaConsumerLagObservers.Delete(o.id)
	})
}

func (o *HGConsumerLagObserver) hgTopicLags() map[string]int64 {
	o.mu.RLock()
	defer o.mu.RUnlock()
	lags := make(map[string]int64, len(o.partitions))
	for topic, partitions := range o.partitions {
		lags[topic] = 0
		for _, state := range partitions {
			if state.initialized && state.highWatermark > state.nextOffset {
				lags[topic] += state.highWatermark - state.nextOffset
			}
		}
	}
	return lags
}

// OnProduceRecordUnbuffered 统计最终完成的生产记录和失败数。
func (HGMetricHook) OnProduceRecordUnbuffered(_ *kgo.Record, err error) {
	hgKafkaProduceRecords.Add(1)
	if err != nil {
		hgKafkaProduceFailures.Add(1)
	}
}

// OnProduceBatchWritten 统计成功写入 broker 的批次数。
func (HGMetricHook) OnProduceBatchWritten(_ kgo.BrokerMetadata, _ string, _ int32, _ kgo.ProduceBatchMetrics) {
	hgKafkaWrittenBatches.Add(1)
}

// OnGroupManageError 统计导致消费组会话退出并退避的错误。
func (HGMetricHook) OnGroupManageError(_ error) {
	hgKafkaGroupErrors.Add(1)
}

func hgObserveConsumeBatch(records int, elapsed time.Duration) {
	if records <= 0 {
		return
	}
	hgKafkaConsumeBatches.Add(1)
	hgKafkaConsumeBatchRecords.Add(uint64(records))
	hgKafkaConsumeBatchNanos.Add(uint64(elapsed))
}

func hgObserveCommit(partitions int, elapsed time.Duration, err error) {
	hgKafkaCommits.Add(1)
	hgKafkaCommitPartitions.Add(uint64(partitions))
	hgKafkaCommitNanos.Add(uint64(elapsed))
	if err != nil {
		hgKafkaCommitFailures.Add(1)
	}
}

func hgKafkaConsumerLagSeries() map[string]int64 {
	series := make(map[string]int64)
	hgKafkaConsumerLagObservers.Range(func(_, value any) bool {
		observer, ok := value.(*HGConsumerLagObserver)
		if !ok || observer == nil {
			return true
		}
		for topic, lag := range observer.hgTopicLags() {
			series[observer.group+"\x00"+topic] += lag
		}
		return true
	})
	return series
}

// HGConsumerLagSnapshot 是按消费组和 topic 聚合的应用处理 lag，不暴露 partition 等高基数维度。
type HGConsumerLagSnapshot struct {
	Group      string
	Topic      string
	LagRecords int64
}

// HGConsumerLagSnapshots 返回排序后的只读快照；该值不是 Kafka Admin API committed-offset lag。
func HGConsumerLagSnapshots() []HGConsumerLagSnapshot {
	series := hgKafkaConsumerLagSeries()
	keys := make([]string, 0, len(series))
	for key := range series {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]HGConsumerLagSnapshot, 0, len(keys))
	for _, key := range keys {
		separator := strings.IndexByte(key, 0)
		result = append(result, HGConsumerLagSnapshot{Group: key[:separator], Topic: key[separator+1:], LagRecords: series[key]})
	}
	return result
}

// HGAssignedPartitionsSnapshot 返回当前进程已分配 partition 数，不执行 broker I/O。
func HGAssignedPartitionsSnapshot() int64 { return hgKafkaAssignedPartitionGauge.Load() }

func hgKafkaOnPartitionsAssigned(_ context.Context, _ *kgo.Client, partitions map[string][]int32) {
	count := hgPartitionCount(partitions)
	hgKafkaPartitionsAssigned.Add(uint64(count))
	hgKafkaAssignedPartitionGauge.Add(int64(count))
}

func hgKafkaOnPartitionsRevoked(_ context.Context, _ *kgo.Client, partitions map[string][]int32) {
	count := hgPartitionCount(partitions)
	hgKafkaPartitionsRevoked.Add(uint64(count))
	hgKafkaAssignedPartitionGauge.Add(-int64(count))
}

func hgKafkaOnPartitionsLost(_ context.Context, _ *kgo.Client, partitions map[string][]int32) {
	count := hgPartitionCount(partitions)
	hgKafkaPartitionsLost.Add(uint64(count))
	hgKafkaAssignedPartitionGauge.Add(-int64(count))
}

// HGConsumerLagObserverOpts 将原有分区计数与 observer 的 revoke/lost 清理组合到同一组 franz-go 回调。
func HGConsumerLagObserverOpts(observer *HGConsumerLagObserver) []kgo.Opt {
	return []kgo.Opt{
		kgo.OnPartitionsAssigned(hgKafkaOnPartitionsAssigned),
		kgo.OnPartitionsRevoked(func(ctx context.Context, client *kgo.Client, partitions map[string][]int32) {
			hgKafkaOnPartitionsRevoked(ctx, client, partitions)
			observer.ObservePartitionsRevoked(partitions)
		}),
		kgo.OnPartitionsLost(func(ctx context.Context, client *kgo.Client, partitions map[string][]int32) {
			hgKafkaOnPartitionsLost(ctx, client, partitions)
			observer.ObservePartitionsRevoked(partitions)
		}),
	}
}

func hgPartitionCount(partitions map[string][]int32) int {
	count := 0
	for _, values := range partitions {
		count += len(values)
	}
	return count
}

// HGKafkaMetricsHandler 返回 Prometheus text exposition 格式的内存指标，不访问外部依赖。
func HGKafkaMetricsHandler(componentWriters ...func(io.Writer)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")

		writeCounter := func(name string, help string, value uint64) {
			_, _ = fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s counter\n", name, help, name)
			_, _ = fmt.Fprintf(w, "%s %d\n", name, value)
		}
		writeCounter("mlc_kafka_produce_records_total", "Kafka records completed by producers.", hgKafkaProduceRecords.Load())
		writeCounter("mlc_kafka_produce_failures_total", "Kafka records that failed production.", hgKafkaProduceFailures.Load())
		writeCounter("mlc_kafka_produce_batches_total", "Kafka batches written to brokers.", hgKafkaWrittenBatches.Load())
		writeCounter("mlc_kafka_consume_batches_total", "Kafka record batches handled by consumers.", hgKafkaConsumeBatches.Load())
		writeCounter("mlc_kafka_consume_batch_records_total", "Kafka records observed in handled batches.", hgKafkaConsumeBatchRecords.Load())
		writeCounter("mlc_kafka_consume_batch_duration_nanoseconds_total", "Total consumer batch handling duration in nanoseconds.", hgKafkaConsumeBatchNanos.Load())
		writeCounter("mlc_kafka_fetch_errors_total", "Kafka fetch errors.", hgKafkaFetchErrors.Load())
		writeCounter("mlc_kafka_handler_failures_total", "Domain event handler failures.", hgKafkaHandlerFailures.Load())
		writeCounter("mlc_kafka_dlq_writes_total", "Kafka DLQ writes.", hgKafkaDLQWrites.Load())
		writeCounter("mlc_kafka_dlq_failures_total", "Kafka DLQ write failures.", hgKafkaDLQFailures.Load())
		writeCounter("mlc_kafka_dlq_write_successes_total", "Kafka DLQ records confirmed by the producer.", hgKafkaDLQSuccesses.Load())
		writeCounter("mlc_kafka_retryable_failures_total", "Retryable Kafka handler failures.", hgKafkaRetryableFailures.Load())
		writeCounter("mlc_kafka_terminal_failures_total", "Terminal Kafka handler failures.", hgKafkaTerminalFailures.Load())
		writeCounter("mlc_kafka_commits_total", "Kafka offset commit attempts.", hgKafkaCommits.Load())
		writeCounter("mlc_kafka_commit_failures_total", "Kafka offset commit failures.", hgKafkaCommitFailures.Load())
		writeCounter("mlc_kafka_commit_partitions_total", "Kafka partitions included in offset commits.", hgKafkaCommitPartitions.Load())
		writeCounter("mlc_kafka_commit_duration_nanoseconds_total", "Total Kafka offset commit duration in nanoseconds.", hgKafkaCommitNanos.Load())
		writeCounter("mlc_kafka_consumer_panics_total", "Recovered domain event handler panics.", hgKafkaConsumerPanics.Load())
		writeCounter("mlc_kafka_group_errors_total", "Kafka consumer group management errors.", hgKafkaGroupErrors.Load())
		writeCounter("mlc_kafka_partitions_assigned_total", "Kafka partitions assigned to this process.", hgKafkaPartitionsAssigned.Load())
		writeCounter("mlc_kafka_partitions_revoked_total", "Kafka partitions revoked from this process.", hgKafkaPartitionsRevoked.Load())
		writeCounter("mlc_kafka_partitions_lost_total", "Kafka partitions lost by this process.", hgKafkaPartitionsLost.Load())
		_, _ = fmt.Fprint(w, "# HELP mlc_kafka_assigned_partitions Current Kafka partitions assigned to this process.\n# TYPE mlc_kafka_assigned_partitions gauge\n")
		_, _ = fmt.Fprintf(w, "mlc_kafka_assigned_partitions %d\n", hgKafkaAssignedPartitionGauge.Load())
		_, _ = fmt.Fprint(w, "# HELP mlc_kafka_consumer_lag_records Latest observed Kafka consumer lag in records.\n# TYPE mlc_kafka_consumer_lag_records gauge\n")
		lagSeries := hgKafkaConsumerLagSeries()
		keys := make([]string, 0, len(lagSeries))
		for key := range lagSeries {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			separator := strings.IndexByte(key, 0)
			group, topic := key[:separator], key[separator+1:]
			_, _ = fmt.Fprintf(w, "mlc_kafka_consumer_lag_records{group=%s,topic=%s} %d\n", strconv.Quote(group), strconv.Quote(topic), lagSeries[key])
		}
		for _, writeMetrics := range componentWriters {
			if writeMetrics != nil {
				writeMetrics(w)
			}
		}
	})
}

func hgResetKafkaMetricsForTest() {
	hgKafkaProduceRecords.Store(0)
	hgKafkaProduceFailures.Store(0)
	hgKafkaWrittenBatches.Store(0)
	hgKafkaConsumeBatches.Store(0)
	hgKafkaConsumeBatchRecords.Store(0)
	hgKafkaConsumeBatchNanos.Store(0)
	hgKafkaFetchErrors.Store(0)
	hgKafkaHandlerFailures.Store(0)
	hgKafkaDLQWrites.Store(0)
	hgKafkaDLQFailures.Store(0)
	hgKafkaDLQSuccesses.Store(0)
	hgKafkaRetryableFailures.Store(0)
	hgKafkaTerminalFailures.Store(0)
	hgKafkaConsumerLagObservers = sync.Map{}
	hgKafkaCommits.Store(0)
	hgKafkaCommitFailures.Store(0)
	hgKafkaCommitPartitions.Store(0)
	hgKafkaCommitNanos.Store(0)
	hgKafkaConsumerPanics.Store(0)
	hgKafkaGroupErrors.Store(0)
	hgKafkaPartitionsAssigned.Store(0)
	hgKafkaPartitionsRevoked.Store(0)
	hgKafkaPartitionsLost.Store(0)
	hgKafkaAssignedPartitionGauge.Store(0)
}

// HGMetricsSnapshot 保留原有轻量快照接口。
func HGMetricsSnapshot() (produceRecords uint64, writtenBatches uint64) {
	return hgKafkaProduceRecords.Load(), hgKafkaWrittenBatches.Load()
}
