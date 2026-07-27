package HGKafkaPackage

import (
	"context"
	"fmt"
	"io"
	"net/http"
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
