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

var (
	// HGGlobalKgoClient 是业务集群的全局 franz-go Client。
	//
	// franz-go 的 Client 同时支持生产和消费；复用单 Client 可以显著降低服务实例到 broker 的连接数量。
	// 高并发部署时仍建议按业务域创建少量长生命周期 Client，不要在请求内创建 Client。
	HGGlobalKgoClient *kgo.Client
	hgClientMu        sync.RWMutex
)

// HGInitKafka 初始化业务 Kafka Client。
//
// 该函数应在程序启动阶段调用一次；若初始化失败，调用方应阻止服务启动，避免请求期才暴露 MQ 不可用。
// 当前项目 Go 版本为 1.23.5，kadm 新版本要求更高 Go 版本，因此这里先接入 kgo 核心生产/消费能力。
func HGInitKafka(cfg HGKafkaClusterConfig) error {
	opts, err := HGNewBusinessProducerOpts(cfg.Business)
	if err != nil {
		return fmt.Errorf("build business kafka opts: %w", err)
	}

	client, err := kgo.NewClient(opts...)
	if err != nil {
		return fmt.Errorf("new business kafka client: %w", err)
	}

	hgClientMu.Lock()
	oldClient := HGGlobalKgoClient
	HGGlobalKgoClient = client
	hgClientMu.Unlock()

	if oldClient != nil {
		oldClient.Close()
	}

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
//
// 同步发送适合必须确认入 Kafka 后才能返回的核心链路；调用方必须传入有超时/取消能力的 ctx，避免 broker 异常时请求无限等待。
func HGSendBusinessEvent(ctx context.Context, topic string, key string, data any) error {
	record, err := HGBuildRecord(ctx, topic, key, data)
	if err != nil {
		return err
	}

	client := HGClient()
	if client == nil {
		return errors.New("kafka client is not initialized")
	}

	if err := client.ProduceSync(ctx, record).FirstErr(); err != nil {
		return fmt.Errorf("produce business kafka event topic=%s: %w", topic, err)
	}

	return nil
}

// HGSendLogEvent 异步发送日志/埋点事件。
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

	client.Produce(ctx, record, func(r *kgo.Record, err error) {
		if err == nil {
			return
		}
		logHG.ErrFInfo("produce kafka log event failed topic=%s err=%v", topic, err)
		if dlqErr := HGSendDLQ(ctx, r, "log", err.Error()); dlqErr != nil {
			logHG.ErrFInfo("send kafka log dlq failed topic=%s err=%v", topic, dlqErr)
		}
	})
}

// HGClient 返回当前全局 Kafka Client。
func HGClient() *kgo.Client {
	hgClientMu.RLock()
	defer hgClientMu.RUnlock()
	return HGGlobalKgoClient
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.Flush(ctx); err != nil {
		logHG.ErrFInfo("flush kafka client failed err=%v", err)
	}
	client.Close()
}
