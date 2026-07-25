package HGKafkaPackage

import (
	UtilsPackage "MLC_GO/internal/pkg/utils"
	"context"

	"github.com/twmb/franz-go/pkg/kgo"
)

const hgTraceHeaderTID = "x-hg-tid"

// HGInjectTraceToRecord 把请求链路 TID 写入 Kafka header。
//
// Kafka 消息跨服务后不再共享 HTTP context，因此必须把最小链路标识放入 header；不要在 header 中写入 token、手机号等敏感信息。
func HGInjectTraceToRecord(ctx context.Context, record *kgo.Record) {
	if ctx == nil || record == nil {
		return
	}

	tid := UtilsPackage.GetTID(ctx)
	if tid == "" {
		return
	}

	record.Headers = append(record.Headers, kgo.RecordHeader{
		Key:   hgTraceHeaderTID,
		Value: []byte(tid),
	})
}

// HGExtractTraceFromRecord 从 Kafka header 恢复 TID 到 context。
//
// 消费端 handler 应使用返回的 context 写日志、调用下游服务，保证消息链路可追踪。
func HGExtractTraceFromRecord(record *kgo.Record) context.Context {
	return HGExtractTraceFromRecordContext(context.Background(), record)
}

// HGExtractTraceFromRecordContext 在保留父 context 取消、deadline 和值的基础上恢复 Kafka trace。
func HGExtractTraceFromRecordContext(ctx context.Context, record *kgo.Record) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if record == nil {
		return ctx
	}

	for _, header := range record.Headers {
		if header.Key == hgTraceHeaderTID && len(header.Value) > 0 {
			return UtilsPackage.InjectTID(ctx, string(header.Value))
		}
	}

	return ctx
}

// HGTraceHook 是 franz-go hook 占位实现。
//
// 当前链路透传在 record header 层完成；保留 hook 类型用于后续接入 OpenTelemetry/Prometheus 时不改调用方代码。
type HGTraceHook struct{}

// OnNewClient 让 HGTraceHook 满足 franz-go HookNewClient 接口。
//
// trace header 注入发生在发送前，这里只作为扩展点，未来可以在 client 初始化时挂接全链路追踪资源。
func (HGTraceHook) OnNewClient(_ *kgo.Client) {}
