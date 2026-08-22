/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-07-04 16:36:21
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-08-22 20:51:33
 * @FilePath: /MLC_GO/internal/pkg/kafka/hg_trace.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
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

// HGExtractTraceFromRecordContext 从 Kafka 消息的 Header 中提取 TraceID，注入到 context.Context，实现跨进程链路追踪传递。
// 典型场景：上游服务发 Kafka 消息时把 traceId 放进 kafka header；消费端收到消息，调用本函数拿到 traceId，后续整个消费协程的 ctx 就带上链路 ID，日志、监控、jaeger 可以复用这条链路。
//
//	@param ctx
//	@param record
//	@return context.Context
func HGExtractTraceFromRecordContext(ctx context.Context, record *kgo.Record) context.Context {
	if ctx == nil {//context.Context 禁止 nil 使用，如果外部传入nil ctx，兜底创建context.Background()，防止后续 panic。
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
