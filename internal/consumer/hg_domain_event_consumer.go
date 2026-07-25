package consumer

import (
	"MLC_GO/internal/events"
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// ErrHandlerNotImplemented 表示消费者识别该事件，但业务投影尚未实现。
// 该错误必须阻止 offset 提交，避免 TODO Handler 静默吞掉领域事件。
var ErrHandlerNotImplemented = errors.New("consumer handler not implemented")

// NewHandlerNotImplementedError 返回带消费者和事件上下文的可识别错误。
func NewHandlerNotImplementedError(consumerName string, eventName string) error {
	return fmt.Errorf("%w: consumer=%s event=%s", ErrHandlerNotImplemented, consumerName, eventName)
}

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
