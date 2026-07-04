/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-07-04 16:34:42
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-07-04 17:16:24
 * @FilePath: /MLC_GO/internal/pkg/kafka/hg_config_builder.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE

 * 两套配置：高吞吐埋点 / 高可靠交易
 */

package HGKafkaPackage

import (
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

const (
	hgDefaultMaxBufferedRecords = 100000
	hgDefaultMaxBufferedBytes   = 512 * 1024 * 1024
	hgLogBatchMaxBytes          = 16 * 1024 * 1024
	hgBusinessBatchMaxBytes     = 8 * 1024 * 1024
)

// HGNewLogProducerOpts 构建日志/埋点集群的生产者配置；埋点集群：极致吞吐，acks=1，高批量
//
// 设计目标：吞吐优先、延迟可控、内存有上限。
// - acks=1：leader 写入即确认，吞吐高，但 broker 故障窗口内存在少量丢失风险。
// - lz4：压缩速度快，适合大流量日志；franz-go v1.16.0 的压缩在 kgo 包内，不需要文档中的 kcompress 包。
// - MaxBufferedRecords/Bytes：显式限制客户端缓冲，避免千万并发突刺把进程内存打爆。
// - ProducerLinger：小幅聚合批次，提升网卡与 broker 吞吐；过大将增加尾延迟。
func HGNewLogProducerOpts(cfg HGClusterConfig) ([]kgo.Opt, error) {
	normalized, err := HGBuildClusterConfig(cfg)
	if err != nil {
		return nil, err
	}
	normalized.Acks = HGAcksLeader

	return hgNewProducerOpts(normalized, hgLogBatchMaxBytes, 50*time.Millisecond), nil
}

// HGNewBusinessProducerOpts 构建业务事件集群的生产者配置。
//
// 设计目标：可靠性优先，兼顾批量吞吐。
// - acks=all：ISR 副本全部确认才成功，降低 leader 故障导致的数据丢失风险。
// - franz-go 默认启用幂等写入，不手动关闭；这能降低重试导致的重复写入风险。
// - retry 由配置控制，但生产 Exactly Once 仍需业务幂等键、事务/本地消息表等配合。
func HGNewBusinessProducerOpts(cfg HGClusterConfig) ([]kgo.Opt, error) {
	normalized, err := HGBuildClusterConfig(cfg)
	if err != nil {
		return nil, err
	}
	if normalized.Acks == "" {
		normalized.Acks = HGAcksAll
	}

	return hgNewProducerOpts(normalized, hgBusinessBatchMaxBytes, 100*time.Millisecond), nil
}

func hgNewProducerOpts(cfg HGClusterConfig, batchMaxBytes int32, linger time.Duration) []kgo.Opt {
	opts := []kgo.Opt{
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.RequiredAcks(hgToKgoAcks(cfg.Acks)),
		kgo.ProducerBatchCompression(kgo.Lz4Compression(), kgo.SnappyCompression(), kgo.NoCompression()),
		kgo.ProducerBatchMaxBytes(batchMaxBytes),
		// kgo.ProducerBatchMaxDuration(50), // 50ms强制刷批
		kgo.ProducerLinger(linger),
		kgo.RecordRetries(cfg.Retry),
		kgo.MaxBufferedRecords(hgDefaultMaxBufferedRecords),
		kgo.MaxBufferedBytes(hgDefaultMaxBufferedBytes),
		kgo.ProduceRequestTimeout(10 * time.Second),
		// 开启幂等生产者，防止重试重复
		//kgo.IdempotentProducer(),
		kgo.WithHooks(HGMetricHook{}, HGTraceHook{}),
	}

	if cfg.ClientID != "" {
		// 设置客户端ID
		opts = append(opts, kgo.ClientID(cfg.ClientID))
		// 为啥没有这个：注入监控、链路钩子
		// opts = append(opts, MetricHookOpt(), TraceHookOpt())
	}

	return opts
}

func hgToKgoAcks(acks string) kgo.Acks {
	switch acks {
	case HGAcksNone:
		return kgo.NoAck()
	case HGAcksLeader:
		return kgo.LeaderAck()
	default:
		return kgo.AllISRAcks()
	}
}
