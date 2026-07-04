package HGKafkaPackage

import (
	"sync/atomic"

	"github.com/twmb/franz-go/pkg/kgo"
)

var (
	hgKafkaBufferedRecords atomic.Uint64
	hgKafkaWrittenBatches  atomic.Uint64
)

// HGMetricHook 提供轻量级 franz-go hook，用于统计客户端侧缓冲和写出批次数。
//
// 这里不直接引入 Prometheus 依赖，避免扩大项目观测体系边界；生产接入时可在这些 hook 中桥接现有 metrics registry。
type HGMetricHook struct{}

// OnProduceRecordBuffered 在消息进入 franz-go 客户端缓冲区时触发。
func (HGMetricHook) OnProduceRecordBuffered(_ *kgo.Record) {
	hgKafkaBufferedRecords.Add(1)
}

// OnProduceBatchWritten 在一个 produce batch 写入 broker 连接后触发。
func (HGMetricHook) OnProduceBatchWritten(_ kgo.BrokerMetadata, _ string, _ int32, _ kgo.ProduceBatchMetrics) {
	hgKafkaWrittenBatches.Add(1)
}

// HGMetricsSnapshot 返回 Kafka 客户端侧轻量指标快照，便于单测或健康检查读取。
func HGMetricsSnapshot() (bufferedRecords uint64, writtenBatches uint64) {
	return hgKafkaBufferedRecords.Load(), hgKafkaWrittenBatches.Load()
}
