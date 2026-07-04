package consumer

import (
	"MLC_GO/internal/events"
	"context"
	"encoding/json"
	"fmt"
)

// Handler 处理一种或多种领域事件。
// Feed/Search/Statistic/Audit 各自实现自己的 Handler，互不影响、互不共享消费位点。
type Handler interface {
	// Handle 处理统一事件外壳；实现方根据 EventName 决定是否消费该事件。
	Handle(ctx context.Context, envelope events.EventEnvelope) error
}

// DecodeEnvelope 把 Kafka value 解成统一事件外壳。
func DecodeEnvelope(value []byte) (events.EventEnvelope, error) {
	var envelope events.EventEnvelope
	// Kafka value 必须是 EventEnvelope JSON；payload 内部才是具体业务事件。
	if err := json.Unmarshal(value, &envelope); err != nil {
		return envelope, fmt.Errorf("decode domain event envelope: %w", err)
	}
	if envelope.EventName == "" {
		// EventName 是消费侧路由的最小字段，缺失时不能安全分发。
		return envelope, fmt.Errorf("domain event name cannot be empty")
	}
	return envelope, nil
}
