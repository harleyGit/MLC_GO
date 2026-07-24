/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-07-04 16:36:21
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-07-23 17:37:35
 * @FilePath: /MLC_GO/internal/pkg/kafka/hg_metric_hook.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 
 功能：prometheus埋点钩子（发送成功/lag/耗时）
 */

package HGKafkaPackage

import (
	"sync/atomic"

	"github.com/twmb/franz-go/pkg/kgo"
)

//TODO：大厂除了统计下面的，还要统计消息数量、失败数量、延迟、DLQ数量、Consumer Lag，用于 Producer性能监控 + 容量评估 + 故障定位。
var (

	// hgKafkaBufferedRecords 定义一个原子计数器，为Kafka 当前缓冲中的消息数量
	hgKafkaBufferedRecords atomic.Uint64

	// hgKafkaWrittenBatches  已经写入 Kafka 的批次数量。
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
// 获取当前 Kafka 指标快照
//	@return bufferedRecords 
//	@return writtenBatches 
func HGMetricsSnapshot() (bufferedRecords uint64, writtenBatches uint64) {

	// return Load() 读取原子变量，线程安全
	return hgKafkaBufferedRecords.Load(), hgKafkaWrittenBatches.Load()
}
