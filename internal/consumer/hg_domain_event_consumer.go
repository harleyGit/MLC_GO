/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-07-04 18:26:03
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-07-30 16:48:41
 * @FilePath: /MLC_GO/internal/consumer/hg_domain_event_consumer.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package consumer

import (
	"MLC_GO/internal/events"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
)

type hgDeliveryContextKey struct{}

// Delivery 描述 Kafka 消息的稳定来源坐标，用于按 topic/partition/offset 做有界永久幂等。
type Delivery struct {
	Topic     string
	Partition int32
	Offset    int64
}

// WithDelivery 把 Kafka 来源坐标附加到业务 Handler context。
func WithDelivery(ctx context.Context, delivery Delivery) context.Context {
	return context.WithValue(ctx, hgDeliveryContextKey{}, delivery)
}

// DeliveryFromContext 读取 Kafka 来源坐标。
func DeliveryFromContext(ctx context.Context) (Delivery, bool) {
	if ctx == nil {
		return Delivery{}, false
	}
	delivery, ok := ctx.Value(hgDeliveryContextKey{}).(Delivery)
	return delivery, ok
}

// ErrHandlerNotImplemented 表示消费者识别该事件，但业务投影尚未实现。
// 该错误必须阻止 offset 提交，避免 TODO Handler 静默吞掉领域事件。
var ErrHandlerNotImplemented = errors.New("consumer handler not implemented")

// ErrUnsupportedEnvelopeVersion 表示消息使用了当前进程无法安全解释的未来协议版本。
var ErrUnsupportedEnvelopeVersion = errors.New("unsupported domain event envelope version")

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
	if envelope.Version == 0 {
		// 升级前的历史 Envelope 没有显式版本，按最早稳定协议 v1 解读。
		envelope.Version = events.EventVersionV1
	}
	if envelope.Version != events.EventVersionV1 {
		return envelope, fmt.Errorf("%w: version=%d", ErrUnsupportedEnvelopeVersion, envelope.Version)
	}
	if envelope.EventID == "" {
		// 兼容升级前已持久化到 Outbox/Kafka 的旧 Envelope。完整消息字节的 SHA-256 在重试和重放时稳定，
		// 仅作为历史数据过渡 ID；新事件始终由生产侧生成 UUID，避免不同合法事件被 EventKey 合并。
		envelope.EventID = fmt.Sprintf("legacy-%x", sha256.Sum256(value))
	}
	return envelope, nil
}
