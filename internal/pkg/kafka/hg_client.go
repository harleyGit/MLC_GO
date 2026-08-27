// package hg_client.go:全局生产者单例、发送通用方法
// 单 Client 同时支持生产 + 消费，无需区分 producer/consumer 两套连接，大幅减少 Broker 连接数（大厂集群连接管控核心优化）
package HGKafkaPackage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"MLC_GO/internal/pkg/logHG"

	"github.com/twmb/franz-go/pkg/kgo"
)

const (
	// hgKafkaStartupPingTimeout 是启动期 Kafka 可达性探测上限。
	// kgo.NewClient 只构建本地 client，不代表 broker 已可达；启动期 Ping 可以提前发现 broker 地址错误、网络不通或认证链路异常。
	// 这里不用过长超时，是为了让容器编排系统快速重启/报警，而不是让进程长期卡在启动阶段。
	hgKafkaStartupPingTimeout = 3 * time.Second
)

var (
	// HGGlobalKgoClient 是业务集群的全局 franz-go Client。
	//
	// franz-go 的 Client 同时支持生产和消费；复用单 Client 可以显著降低服务实例到 broker 的连接数量。
	// 高并发部署时仍建议按业务域创建少量长生命周期 Client，不要在请求内创建 Client。
	HGGlobalKgoClient *kgo.Client
	hgClientMu        sync.RWMutex
)

// HGInitKafka 初始化业务 Kafka Client。
// HGInitKafka 程序启动初始化全局客户端
//
// 该函数应在程序启动阶段调用一次；若初始化失败，调用方应阻止服务启动，避免请求期才暴露 MQ 不可用。
// 当前项目 Go 版本为 1.23.5，kadm 新版本要求更高 Go 版本，因此这里先接入 kgo 核心生产/消费能力。
//
// 初始化分三步：
// 1. 根据业务集群配置构建 producer/consumer 共用的 kgo opts。
// 2. 创建临时 client 后立即 Ping broker，确认配置不是“看起来合法但实际不可用”。
// 3. Ping 成功后再替换全局 client，避免失败初始化把 HGGlobalKgoClient 置为不可用实例。
func HGInitKafka(cfg HGKafkaClusterConfig) error {
	// 业务客户端（生产+消费共用）
	opts, err := HGNewBusinessProducerOpts(cfg.Business)
	if err != nil {
		return fmt.Errorf("build business kafka opts: %w", err)
	}

	// 创建 Kafka 客户端
	client, err := kgo.NewClient(opts...)
	if err != nil {
		return fmt.Errorf("new business kafka client: %w", err)
	}
	if err := hgPingKafkaClient(client, hgKafkaStartupPingTimeout); err != nil {
		// Ping 失败说明当前 client 还没有进入可服务状态；直接关闭临时 client，保持全局状态不变。
		client.Close()
		return fmt.Errorf("ping business kafka client: %w", err)
	}

	hgClientMu.Lock()
	oldClient := HGGlobalKgoClient
	HGGlobalKgoClient = client
	hgClientMu.Unlock()

	if oldClient != nil {
		// 热替换或测试重复初始化时释放旧 client，避免旧连接继续占用 broker 连接数和本地资源。
		oldClient.Close()
	}
	// 埋点客户端可独立创建，或复用同一client（看流量隔离需求）
	return nil
}

// HGBuildRecord 将任意业务数据序列化为 Kafka Record，并注入链路追踪 header。
//
// key 用于 Kafka 分区路由；对同一实体强顺序的事件，应使用稳定业务 key，例如 order_id/user_id。
func HGBuildRecord(ctx context.Context, topic string, key string, data any) (*kgo.Record, error) {
	if topic == "" {
		return nil, errors.New("kafka topic cannot be empty")
	}

	val, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal kafka payload: %w", err)
	}

	record := &kgo.Record{
		Topic: topic,
		Key:   []byte(key),
		Value: val,
	}
	HGInjectTraceToRecord(ctx, record)

	return record, nil
}

// HGSendBusinessEvent 同步发送高可靠业务事件。
// /service层调用发送业务事件
//
// 同步发送适合必须确认入 Kafka 后才能返回的核心链路；调用方必须传入有超时/取消能力的 ctx，避免 broker 异常时请求无限等待。
func HGSendBusinessEvent(ctx context.Context, topic string, key string, data any) error {
	// 把 data（envelope 结构体）序列化为 kafka 消息 value，组装成 kgo 的`*kgo.Record`
	record, err := HGBuildRecord(ctx, topic, key, data)
	if err != nil {
		return err
	}

	// 获取全局 kafka kgo 客户端
	client := HGClient()
	if client == nil {
		return errors.New("kafka client is not initialized")
	}

	// 把一条消息 record 同步发送到 Kafka，等待 Kafka 返回结果，然后获取第一个错误
	if err := client.ProduceSync(ctx, record).FirstErr(); err != nil {
		return fmt.Errorf("produce business kafka event topic=%s: %w", topic, err)
	}
	// TODO: 异步发送 下面没有做，可以看下如何做he kafka
	/*
		// 注入trace到header
		InjectTraceToRecord(ctx, record)
		// 同步发送；超高吞吐用ProduceAsync非阻塞
		return GlobalKgoClient.ProduceSync(ctx, record).FirstErr()
	*/
	return nil
}

// HGSendBusinessEventBytes 发送已经序列化好的业务事件字节。
// Outbox dispatcher 使用该方法，避免把 outbox.payload 再次 JSON Marshal 成字符串。
func HGSendBusinessEventBytes(ctx context.Context, topic string, key string, payload []byte) error {
	if topic == "" {
		return errors.New("kafka topic cannot be empty")
	}
	record := &kgo.Record{Topic: topic, Key: []byte(key), Value: payload}
	HGInjectTraceToRecord(ctx, record)

	client := HGClient()
	if client == nil {
		return errors.New("kafka client is not initialized")
	}
	if err := client.ProduceSync(ctx, record).FirstErr(); err != nil {
		return fmt.Errorf("produce business kafka event topic=%s: %w", topic, err)
	}
	return nil
}

// HGSendLogEvent 异步发送日志/埋点事件。不阻塞业务接口
//
// 异步发送不阻塞主业务链路；若发送失败，会尝试进入 DLQ。注意：当进程崩溃时，尚未 flush 的异步消息仍可能丢失。
func HGSendLogEvent(ctx context.Context, topic string, data any) {
	record, err := HGBuildRecord(ctx, topic, "", data)
	if err != nil {
		logHG.ErrFInfo("build kafka log event failed topic=%s err=%v", topic, err)
		return
	}

	client := HGClient()
	if client == nil {
		logHG.ErrFInfo("kafka client is not initialized topic=%s", topic)
		return
	}

	// Produce 的核心作用：异步发送 Kafka 消息，不阻塞当前业务 goroutine，通过 callback 异步通知发送结果
	client.Produce(ctx, record, func(r *kgo.Record, err error) { // callback是发送结果回调，若是kafka写成功，直接结束
		if err == nil {
			return
		}
		logHG.ErrFInfo("produce kafka log event failed topic=%s err=%v", topic, err)
		// 发送失败，尝试进入 DLQ【死信队列，作用是：Kafka消息发送失败后，不直接丢弃，而保存到另一个 Topic。】
		if dlqErr := HGSendDLQ(ctx, r, "log", err.Error()); dlqErr != nil {
			logHG.ErrFInfo("send kafka log dlq failed topic=%s err=%v", topic, dlqErr)
		}
	})
}

// TODO：优雅关闭 未做，看看啥原因
// Close 优雅关闭，等待缓存消息全部发送完成
/* func Close() {
	if GlobalKgoClient != nil {
		_ = GlobalKgoClient.Flush(context.Background())
		GlobalKgoClient.Close()
	}
} */

// HGClient 返回当前全局 Kafka Client。
func HGClient() *kgo.Client {
	hgClientMu.RLock()
	defer hgClientMu.RUnlock()
	return HGGlobalKgoClient
}

// HGPingKafka 检查当前全局 Kafka Client 是否能访问任一 broker。
//
// 该方法主要用于 /readyz：只要任一已发现 broker 或 seed broker 能响应 ApiVersions 请求，就认为 Kafka 依赖可达。
// 调用方必须传入带超时/取消的 ctx，避免 ready 探针在网络异常时堆积。
func HGPingKafka(ctx context.Context) error {
	client := HGClient()
	if client == nil {
		return errors.New("kafka client is not initialized")
	}
	return client.Ping(ctx)
}

// hgPingKafkaClient 检测 Kafka Client 是否能够正常连接 Kafka 集群
func hgPingKafkaClient(client *kgo.Client, timeout time.Duration) error {
	// 启动期没有上游请求 ctx，因此使用固定超时的 Background context；cancel 必须释放计时器资源。
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	// 检测 Kafka Client 是否正常，防止宕机、网络不通、DNS失败、SASL认证失败
	return client.Ping(ctx)
}

// HGCloseKafka 优雅关闭 Kafka Client。
//
// 关闭前会尝试 Flush，把客户端缓冲中的异步消息尽量发送出去；生产环境应在服务退出钩子中调用。
func HGCloseKafka() {
	hgClientMu.Lock()
	client := HGGlobalKgoClient
	HGGlobalKgoClient = nil
	hgClientMu.Unlock()

	if client == nil {
		return
	}

	// 服务退出前，等待 Kafka Producer 内部缓存的未发送消息全部发送完成，然后关闭 Kafka Client，避免消息丢失。
	// 时间限制：10秒，避免服务退出时无限等待；若 Kafka broker 不可达，Flush 会在超时后返回错误。如果 Kafka 挂了：Flush()，一直等待，服务无法退出，Pod无法停止
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	//没有 cancel：timer 还会存在。
	defer cancel()
	// 把 Producer 缓冲区里面还没有发送完成的消息全部刷到 Kafka。
	if err := client.Flush(ctx); err != nil {
		logHG.ErrFInfo("flush kafka client failed err=%v", err)
	}
	// 关闭 Kafka Client，释放连接和资源。因为：Flush 只负责发送剩余消息，但是 Client 还有资源，Close负责释放资源
	client.Close()
}
