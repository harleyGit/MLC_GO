package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	// EventVersionV1 是当前领域事件字节协议版本。
	EventVersionV1 = 1
	// SourceServiceMLC 标识事件来源服务，供消费侧做审计、路由和问题排查。
	SourceServiceMLC = "mlc-go"
)

// DomainEvent 是所有领域事件必须实现的最小协议。
// 业务层只依赖该接口和 EventBus，不直接依赖 Kafka producer。
type DomainEvent interface {
	// EventName 返回稳定事件名，消费侧用它做路由分发。
	EventName() string
	// EventKey 返回稳定业务 key，用于 Kafka 分区路由和消费侧幂等。
	EventKey() string
}

// EventMeta 是领域事件公共头。
// 字段保持稳定 JSON tag，便于跨服务按字节协议消费和版本演进。
type EventMeta struct {
	Version       int    `json:"version"`
	TraceID       string `json:"traceId,omitempty"`
	RequestID     string `json:"requestId,omitempty"`
	Timestamp     int64  `json:"timestamp"`
	SourceService string `json:"sourceService"`
}

// EventEnvelope 是写入 Kafka / Outbox 的稳定字节外壳。
//
// 为什么不直接把 Model 发出去：
// 1. Model 会随着数据库字段变化而变化，跨服务消费会被内部表结构绑死。
// 2. EventName/EventKey/Version 是消费者路由、幂等和灰度升级需要的公共协议。
// 3. Payload 保留事件原始 JSON，后续新增字段时老消费者可以忽略未知字段。
type EventEnvelope struct {
	// EventID 是每次领域状态变更的唯一 ID，消费侧用它抵御 Kafka 至少一次投递产生的重复消息。
	EventID       string `json:"eventId"`
	EventName     string `json:"eventName"`
	EventKey      string `json:"eventKey"`
	Version       int    `json:"version"`
	TraceID       string `json:"traceId,omitempty"`
	RequestID     string `json:"requestId,omitempty"`
	Timestamp     int64  `json:"timestamp"`
	SourceService string `json:"sourceService"`
	// Payload json.RawMessage 保留原始事件字节，便于消费侧按 EventName 反序列化成具体业务事件。
	Payload json.RawMessage `json:"payload"`
}

// NewEnvelope 构建事件信封协议【把领域事件封装成统一事件协议。】
// 业务层只构造 VideoReviewedEvent 这类小而稳定的事件，基础设施层再统一补齐事件名、key 和公共元信息。
func NewEnvelope(event DomainEvent) (EventEnvelope, error) {
	if event == nil {
		return EventEnvelope{}, fmt.Errorf("domain event cannot be nil")
	}

	// 先序列化原始事件，Payload 保留业务字段，Envelope 再补统一协议字段。
	payload, err := json.Marshal(event)
	if err != nil {
		return EventEnvelope{}, fmt.Errorf("marshal domain event: %w", err)
	}

	// metaFromEventPayload： 把序列化后的业务 json 再反解析一遍，提取埋点链路信息 TraceID、RequestID 等元数据。
	// 业务 DomainEvent 内部会嵌入 meta 元数据，序列化进 payload 之后，再捞出来放到信封顶层，方便 kafka 消费端不用解析 payload 就能拿到 trace，做链路追踪、日志打印。
	// `EventID` 在发送端生成 UUID，不是 kafka offset。kafka offset 是 kafka 内部的；EventID 是业务层幂等 ID，跨存储（kafka/outbox）全局唯一。
	meta := metaFromEventPayload(payload)
	return EventEnvelope{
		EventID:       uuid.NewString(),  // 全局唯一事件ID，消费端幂等的核心！【消费端幂等的唯一标识，前面 `Consumer.Handle` 就是拿这个 EventID 去 Redis 做幂等计数。】
		EventName:     event.EventName(), // 事件名称，消费端用来判断处理什么事件
		EventKey:      event.EventKey(),  // kafka分区key，业务维度key（比如submissionID）
		Version:       meta.Version,
		TraceID:       meta.TraceID,
		RequestID:     meta.RequestID,
		Timestamp:     meta.Timestamp,
		SourceService: meta.SourceService,
		Payload:       payload, // 原始业务事件 json 字符串； 好处：消费端不需要提前知道每个业务事件结构体；先解析外层信封拿到公共字段（EventID、EventName），判断要不要处理；真正需要业务数据的时候，再 Unmarshal Payload。
	}, nil
	/** 消息最终在 kafka 里面存的数据样子
	{
	  "EventID":"uuid-xxxx",
	  "EventName":"VideoPublished",
	  "EventKey":"sub_12345",
	  "TraceID":"xxx",
	  "RequestID":"xxx",
	  "Payload":"{\"submissionId\":\"xxx\",\"userId\":\"xxx\"}"
	}
	*/
}

// NewEventMeta 创建领域事件公共头。
func NewEventMeta(ctx context.Context) EventMeta {
	return EventMeta{
		Version: EventVersionV1,
		// traceId/requestId 从上游 context 透传，便于 Kafka 消费链路和 HTTP 请求链路关联排查。
		TraceID:       stringValue(ctx, "traceId"),
		RequestID:     stringValue(ctx, "requestId"),
		Timestamp:     time.Now().UTC().UnixMilli(),
		SourceService: SourceServiceMLC,
	}
}

// metaFromEventPayload 从事件 payload 中提取公共头，并补齐默认值。
func metaFromEventPayload(payload []byte) EventMeta {
	var meta EventMeta
	// payload 已由本进程生成；反解失败时继续使用默认 meta，避免因为公共头缺失阻断事件发布。
	_ = json.Unmarshal(payload, &meta)
	if meta.Version == 0 {
		meta.Version = EventVersionV1
	}
	if meta.SourceService == "" {
		meta.SourceService = SourceServiceMLC
	}
	if meta.Timestamp == 0 {
		meta.Timestamp = time.Now().UTC().UnixMilli()
	}
	return meta
}

// stringValue 从 context 中读取字符串值，非字符串或不存在时返回空串。
func stringValue(ctx context.Context, key string) string {
	if ctx == nil {
		return ""
	}
	value, _ := ctx.Value(key).(string)
	return value
}
