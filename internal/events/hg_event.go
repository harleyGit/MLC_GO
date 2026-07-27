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
	EventID       string          `json:"eventId"`
	EventName     string          `json:"eventName"`
	EventKey      string          `json:"eventKey"`
	Version       int             `json:"version"`
	TraceID       string          `json:"traceId,omitempty"`
	RequestID     string          `json:"requestId,omitempty"`
	Timestamp     int64           `json:"timestamp"`
	SourceService string          `json:"sourceService"`
	Payload       json.RawMessage `json:"payload"`
}

// NewEnvelope 把领域事件封装成统一事件协议。
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

	// 事件结构通常内嵌 EventMeta；这里从 payload 反解公共头，兼容未来不同事件结构。
	meta := metaFromEventPayload(payload)
	return EventEnvelope{
		EventID:       uuid.NewString(),
		EventName:     event.EventName(),
		EventKey:      event.EventKey(),
		Version:       meta.Version,
		TraceID:       meta.TraceID,
		RequestID:     meta.RequestID,
		Timestamp:     meta.Timestamp,
		SourceService: meta.SourceService,
		Payload:       payload,
	}, nil
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
