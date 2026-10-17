/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-07-04 16:34:42
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-10-17 11:23:22
 * @FilePath: /MLC_GO/internal/pkg/kafka/hg_config_builder.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE

 * 两套配置：高吞吐埋点 / 高可靠交易
 */

package HGKafkaPackage

import (
	"fmt"
	"strings"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

const (
	// hgDefaultMaxBufferedRecords 限制 producer 本地最多缓存的消息条数，避免 broker 慢或网络抖动时内存无界增长。
	hgDefaultMaxBufferedRecords = 100000
	// hgDefaultMaxBufferedBytes 限制 producer 本地缓存总字节数，和条数上限一起保护进程内存。
	hgDefaultMaxBufferedBytes = 512 * 1024 * 1024
	// hgLogBatchMaxBytes 日志/埋点允许更大批次，以吞吐优先。
	hgLogBatchMaxBytes = 16 * 1024 * 1024
	// hgBusinessBatchMaxBytes 业务事件批次更保守，降低单批失败重试和尾延迟影响。
	hgBusinessBatchMaxBytes = 8 * 1024 * 1024
)

// HGNewLogProducerOpts 构建日志/埋点集群的生产者配置；埋点集群：极致吞吐，acks=1，高批量
//
// 设计目标：吞吐优先、延迟可控、内存有上限。
// - acks=1：leader 写入即确认，吞吐高，但 broker 故障窗口内存在少量丢失风险。
// - lz4：压缩速度快，适合大流量日志；franz-go v1.16.0 的压缩在 kgo 包内，不需要文档中的 kcompress 包。
// - MaxBufferedRecords/Bytes：显式限制客户端缓冲，避免千万并发突刺把进程内存打爆。
// - ProducerLinger：小幅聚合批次，提升网卡与 broker 吞吐；过大将增加尾延迟。
func HGNewLogProducerOpts(cfg HGClusterConfig) ([]kgo.Opt, error) {
	// 先统一清洗 broker、acks、retry 等输入，避免调用方传半成品配置。
	normalized, err := HGBuildClusterConfig(cfg)
	if err != nil {
		return nil, err
	}
	// 日志/埋点允许 leader ack，以吞吐优先；业务事件不能复用该可靠性等级。
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
	// 业务事件配置必须先规范化，再按可靠性策略补默认值。
	normalized, err := HGBuildClusterConfig(cfg)
	if err != nil {
		return nil, err
	}
	if normalized.Acks == "" {
		// 空值默认使用 all，避免核心业务事件因 leader 故障窗口丢失。
		normalized.Acks = HGAcksAll
	}

	return hgNewProducerOpts(normalized, hgBusinessBatchMaxBytes, 100*time.Millisecond), nil
}

// HGNewBusinessClientOpts 构建同时承担业务生产与消费的长生命周期 Client 配置。
// 消费端关闭自动提交，并在每批处理完成后由 consumeLoop 显式提交和释放 rebalance。
func HGNewBusinessClientOpts(cfg HGClusterConfig) ([]kgo.Opt, error) {
	normalized, err := HGBuildClusterConfig(cfg)
	if err != nil {
		return nil, err
	}
	if len(normalized.Topics) == 0 {
		return nil, fmt.Errorf("kafka consumer topics cannot be empty")
	}
	if normalized.GroupID == "" {
		return nil, fmt.Errorf("kafka consumer group_id cannot be empty")
	}

	opts := hgNewProducerOpts(normalized, hgBusinessBatchMaxBytes, 100*time.Millisecond)
	opts = append(opts,
		kgo.ConsumeTopics(normalized.Topics...),
		kgo.ConsumerGroup(normalized.GroupID),
		kgo.DisableAutoCommit(),
		kgo.BlockRebalanceOnPoll(),
	)
	return opts, nil
}

// HGNewBusinessConsumerOpts 构建独立消费组 Client，避免多个读模型共享消费位点。
func HGNewBusinessConsumerOpts(cfg HGClusterConfig, topics []string, groupID string, clientID string) ([]kgo.Opt, error) {
	normalized, err := HGBuildClusterConfig(cfg)
	if err != nil {
		return nil, err
	}
	topics = hgTrimNonEmptyStrings(topics)
	if len(topics) == 0 {
		return nil, fmt.Errorf("kafka consumer topics cannot be empty")
	}
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil, fmt.Errorf("kafka consumer group_id cannot be empty")
	}

	// opts 这是 franz-go（kgo）Kafka 消费者的配置构建函数，针对消费者组（Consumer Group）模式，核心特征是：手动提交 offset + 阻塞式 Rebalance + 分区生命周期回调 + 监控钩子。
	// 和上一段生产者配置对比，这是消费端的对应模板。
	opts := []kgo.Opt{
		// 种子 Broker 列表。 和生产者一样，只用于初次集群发现，连上后 kgo 自动维护完整 broker 元数据。normalized 说明 broker 地址经过了标准化处理（比如去重、补端口、trim 空格）
		kgo.SeedBrokers(normalized.Brokers...),
		/** 订阅的 Topic 列表。
		消费者组模式下，组内所有实例共同消费这些 topic 的所有 partition。
		kgo 会自动协调：每个 partition 只会分配给组内一个消费者实例，实现并行消费。
		如果 topics 为空，这个消费者就不会消费任何东西（调用方应该在传入前做校验）
		*/
		kgo.ConsumeTopics(topics...),
		/** 消费者组 ID。
		这是消费者组模式的核心标识。同一个 groupID 的所有实例构成一个消费组，共同分担 partition。
		Kafka broker（Group Coordinator）根据 groupID 管理：
		组成员关系（谁加入了、谁退出了）
		partition 分配方案
		每个 partition 的消费 offset（存在 __consumer_offsets 内部 topic）
		不同 groupID 互相独立，各自维护自己的 offset，可以实现同一条消息被多个业务消费组分别消费。
		*/
		kgo.ConsumerGroup(groupID),
		/** 不等于自动保证消息不丢失
		这个配置本身只是关闭自动提交 Offset。真正的安全性取决于业务代码是否按照正确顺序执行：
		Kafka 拉取消息
		|
		v
		处理业务
		|
		v
		数据库事务 Commit 成功
		|
		v
		Kafka CommitRecords()
		|
		v
		继续消费

		如果数据库提交成功，但 Kafka Offset 提交失败：
		数据库已经成功写入
			|
			v
		Kafka Offset 未提交
			|
			v
		消费者重启
			|
			v
		同一条消息再次消费
			|
			v
		通过 Inbox EventID 幂等去重


		这正是你前面 hgClaimInbox() 方法的重要用途。
		手动提交 + 数据库幂等处理，是实现可靠的 At-Least-Once（至少一次）消费的一种常见方案。
		*/
		kgo.DisableAutoCommit(),
		/** Poll 期间阻塞 Rebalance。 ⭐ 非常关键的配置
		消费者组发生 rebalance 时（有实例加入 / 退出、订阅 topic 变化），partition 会重新分配。
		默认行为：rebalance 发生时，正在处理的消息可能被打断，partition 被撤走，导致处理到一半的消息被另一个实例重新消费。
		开启 BlockRebalanceOnPoll 后：阻止分区 Rebalance 在应用仍持有当前 Poll 批次的处理权期间完成，但不保证业务处理成功，也不提供事务原子性。
		比如：Poll 获取 100 条消息，处理第 1～50 条，第 51 条处理失败，前 50 条可能已经写入数据库。BlockRebalanceOnPoll() 并不会自动回滚前 50 条数据。
		另外，它通常需要业务在适当时机调用：client.AllowRebalance()，主动释放 Rebalance 阻塞。因此，不能理解成“Poll 执行结束就自动解除阻塞”。
		*/
		kgo.BlockRebalanceOnPoll(),
		/** 监控钩子。 和生产者类似，但这里只挂了 MetricHook，没有 TraceHook。
		采集消费延迟（lag）、消费速率、rebalance 次数、错误率等指标。
		注意：生产者配置里是 HGMetricHook{} + HGTraceHook{} 两个，这里只有一个，可能消费端的 trace 是在业务 handler 里做的，或者消费端暂时不需要 trace。
		*/
		kgo.WithHooks(HGMetricHook{}),
		/** 分区分配回调。 当这个消费者实例被分配到新的 partition 时触发。
		典型用途：
		 - 初始化该 partition 的本地状态（比如缓存、计数器、文件句柄）
		 - 打印日志："被分配了 topic=xxx partition=yyy"
		 - 做一些预热操作
		 - 回调签名通常是 func(ctx context.Context, client *kgo.Client, assigned map[string][]int32)
		*/
		kgo.OnPartitionsAssigned(hgKafkaOnPartitionsAssigned),
		/** 这个回调表示当前 Consumer 的某些分区正在被正常撤销
		最关键的用途：在分区被撤走前，提交当前已处理但未提交的 offset。
		因为手动提交模式下，如果不在这里提交，partition 被撤走后，新的消费者会从上次提交的 offset 开始消费，导致已处理但未提交的消息被重复消费。
		典型流程：Revoked 触发 → flush 本地缓冲 → 提交 offset → 清理资源 → 返回
		这是正常 rebalance 的回调，会等回调执行完才完成 rebalance。
		*/
		kgo.OnPartitionsRevoked(hgKafkaOnPartitionsRevoked),
		/** 分区丢失回调（异常丢失）。 和 Revoked 的区别：
		Revoked：正常 rebalance，你有机会做清理和提交，回调执行完分区才会被撤走。
		Lost：异常情况（比如消费者心跳超时、被 coordinator 踢出组、网络分区），分区已经丢失了，这个回调只是通知你，你来不及提交 offset 了。
		典型用途：
		 - 打告警日志
		 - 清理本地资源（但不要尝试提交 offset，因为已经不是这个 partition 的所有者了，提交可能失败或导致错误）
		 - 重置本地状态
		三个回调的关系：Assigned（拿到）→ Revoked（正常归还，可提交）/ Lost（异常丢失，来不及提交）。正常关闭走 Revoked，崩溃 / 超时走 Lost
		*/
		kgo.OnPartitionsLost(hgKafkaOnPartitionsLost),
	}
	// 先 TrimSpace 去掉首尾空白，再判断非空才设置。
	if clientID = strings.TrimSpace(clientID); clientID != "" {
		opts = append(opts, kgo.ClientID(clientID))
	}
	return opts, nil
}

// hgNewProducerOpts franz-go（kgo）Kafka 生产者的通用配置构建函数，输入集群配置、批量大小、刷批延迟，输出一组 kgo.Opt 选项函数，供 kgo.NewClient(opts...) 使用。
// 核心设计思路：批量攒包 + 压缩 + 重试 + 监控 / 链路钩子，是一个面向高吞吐业务场景的生产者模板。
//
//	@param cfg 集群配置（broker 地址、acks、重试次数、ClientID 等）
//	@param batchMaxBytes 单个批次最大字节数，由调用方传入（不同业务可能不同）
//	@param linger 刷批等待时间，调用方控制（实时性要求高的传小值，吞吐优先传大值）
//	@return []kgo.Opt kgo 的选项是函数式选项模式（Functional Options），每个 kgo.Xxx() 返回一个 Opt 函数，NewClient 时依次应用。
func hgNewProducerOpts(cfg HGClusterConfig, batchMaxBytes int32, linger time.Duration) []kgo.Opt {
	// 通用配置项。一个切片，里面的元素都是实现了 kgo.Opt 接口的函数，用于配置 Kafka 客户端。
	opts := []kgo.Opt{
		/** Seed broker 只用于客户端发现集群，后续 franz-go 会维护完整 broker 元数据。
		种子 Broker 地址列表。
		- 只用于客户端初次连接和集群发现，不需要填全部 broker。
		- 连上任意一个 seed broker 后，kgo 会自动拉取集群元数据（所有 broker、topic、partition 分布），后续自主维护。
		- 所以配置里写 2-3 个 broker 就够了，即使某个 seed 挂了也能通过其他 seed 发现集群。
		*/
		kgo.SeedBrokers(cfg.Brokers...),
		// 生产者发送后的确认机制。 通过 hgToKgoAcks 把自定义配置转换成 kgo 的枚举：
		kgo.RequiredAcks(hgToKgoAcks(cfg.Acks)),
		/** 按优先级尝试压缩，broker/client 不支持时可退回 no compression。
		按优先级尝试压缩，回退机制。
		- kgo 会按顺序尝试：先试 Lz4，如果 broker 或客户端不支持 → 试 Snappy，还不支持 → 不压缩。
		- 为什么选 Lz4 优先：Lz4 压缩 / 解压速度极快，压缩率也不错，是 Kafka 场景的主流选择；Snappy 是老牌备选；NoCompression 是最终兜底。
		- 这个 "按优先级回退" 是 kgo 的特性，比写死单一压缩算法兼容性更好。
		*/
		kgo.ProducerBatchCompression(kgo.Lz4Compression(), kgo.SnappyCompression(), kgo.NoCompression()),
		/** 单个 ProducerBatch 的最大字节数。
		生产者会把发往同一个 partition 的多条消息攒成一个 batch，一次性发送，减少网络请求次数。
		这个值限制 batch 上限，达到就立即发送（不等 linger）。
		⚠️ 注意：这个值不能超过 broker 端的 message.max.bytes，否则 broker 会拒收。通常设 1MB 左右。
		由调用方传入，说明不同业务（比如大消息体的审计日志 vs 小消息的事件）需要不同批次大小。
		*/
		kgo.ProducerBatchMaxBytes(batchMaxBytes),
		// kgo.ProducerBatchMaxDuration(50), // 50ms强制刷批
		/** 刷批延迟（攒批时间）。
		消息到达后，最多等待 linger 时间再发送，期间如果 batch 满了（达到 batchMaxBytes）则提前发。
		linger=0：每条消息立即发（最低延迟，最低吞吐）
		linger=5~50ms：攒一小批再发（吞吐大幅提升，延迟增加几 ms 到几十 ms）
		这是延迟 vs 吞吐的核心调优旋钮，做成参数由调用方决定。
		*/
		kgo.ProducerLinger(linger),
		/** 单条消息最大重试次数。
		发送失败（可重试错误，如 leader 切换、网络抖动）时自动重试。
		配合 RequiredAcks 使用：acks=all 时重试更有意义。
		⚠️ 注意：重试可能导致消息重复（broker 已写入但响应丢失，生产者重试）。如果业务要求 exactly-once，需要开启幂等生产者（下面第 10 点）或业务端去重。
		*/
		kgo.RecordRetries(cfg.Retry),
		/** 最大缓冲区字节数。内存中最大缓冲记录数。
		生产者内部会缓冲待发送的消息（正在攒批、正在重试、飞行中的请求）。
		超过这个数量，Produce 调用会阻塞或返回错误（取决于配置），起到背压（backpressure）作用，防止生产速度远大于发送速度时内存爆掉。*/
		kgo.MaxBufferedRecords(hgDefaultMaxBufferedRecords),
		/** 内存中最大缓冲字节数。
		和上面配套，从字节维度限制内存占用。
		两个限制同时生效，先到哪个触发哪个。
		这是生产者内存防护的双保险。
		*/
		kgo.MaxBufferedBytes(hgDefaultMaxBufferedBytes),
		/** 单次 Produce 请求的超时时间。
		从请求发出去到收到 broker 响应的最大等待时间，10 秒。
		超时后该批次视为失败，进入重试逻辑（如果还有重试次数）。
		注意这是请求级超时，不是整条消息的总超时。总耗时 ≈ retries × (requestTimeout + 退避时间)。
		*/
		kgo.ProduceRequestTimeout(10 * time.Second),
		// 开启幂等生产者，防止重试重复
		//kgo.IdempotentProducer(),
		/** 注入监控和链路追踪钩子。
		kgo 的 Hook 机制可以在消息发送的各个生命周期点（发送前、发送后、元数据更新等）插入回调。
		HGMetricHook：采集发送成功率、延迟、batch 大小等指标，上报到监控系统
		HGTraceHook：注入 trace_id /span_id，把生产链路接入分布式追踪
		这是可观测性的核心入口，所有生产者统一挂载，不需要每个业务自己写。
		*/
		kgo.WithHooks(HGMetricHook{}, HGTraceHook{}),
	}

	// if ClientID 会出现在 broker 的日志和指标里，用于定位是哪个服务实例在生产。
	// 为空就不设置，用 kgo 默认值。
	if cfg.ClientID != "" {
		// ClientID 会进入 broker 日志和指标，用于定位具体服务实例。
		opts = append(opts, kgo.ClientID(cfg.ClientID))
	}

	return opts
}

// hgToKgoAcks 根据 Kafka Producer 的 acks 配置，返回 franz-go 对应的消息确认策略。
// 它决定：Producer 发送消息后，需要 Kafka Broker 返回什么级别的确认，才认为消息发送成功。
// 这是 Kafka 可靠性和性能之间的核心配置。
func hgToKgoAcks(acks string) kgo.Acks {
	switch acks {
	case HGAcksNone:
		// roducer 发消息后，不等待 Kafka 返回确认。可靠性最低，性能最高
		return kgo.NoAck()
	case HGAcksLeader:
		// 等待 Kafka Topic 分区 Leader 写入成功。适合：普通业务：
		return kgo.LeaderAck()
	default:
		// 等待所有 ISR（同步副本）确认。可靠性最高，性能最低。适合：核心业务、交易、订单等。
		return kgo.AllISRAcks()
	}
}
