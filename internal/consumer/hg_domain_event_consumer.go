/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-07-04 18:26:03
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-08-24 21:29:47
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

// 存：把delivery放进ctx
func WithDelivery(ctx context.Context, delivery Delivery) context.Context {
	// context.WithValue 基于父上下文，生成一个新的 context，把 key‑value 附加到新 ctx 上
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

// DeliveredEnvelope 保留批量处理时每条事件各自的 Kafka 来源坐标。
type DeliveredEnvelope struct {
	Delivery Delivery
	Envelope events.EventEnvelope
}

// BatchHandler 在单分区有界批次内按 Kafka 顺序处理领域事件。
type BatchHandler interface {
	HandleBatch(context.Context, []DeliveredEnvelope) error
}

// DecodeEnvelope 是 Kafka 消费侧的领域事件信封解码器。它把 Kafka 消息的 value 字节流反序列化成 events.EventEnvelope，并做了三件事：必填校验、版本兼容、历史数据补 ID。
//	@param value 
//	@return events.EventEnvelope 
//	@return error 
func DecodeEnvelope(value []byte) (events.EventEnvelope, error) {
	var envelope events.EventEnvelope
	// Kafka value 必须是 EventEnvelope JSON；payload 内部才是具体业务事件。
	if err := json.Unmarshal(value, &envelope); err != nil {
		return envelope, fmt.Errorf("decode domain event envelope: %w", err)
	}
	if envelope.EventName == "" {
		// 为空直接报错 —— 消费端靠它做路由分发，没有就不知道该交给哪个 handler。
		return envelope, fmt.Errorf("domain event name cannot be empty")
	}
	if envelope.Version == 0 {
		// 视为升级前的老数据，兜底设为 v1；然后只放行 v1，其他版本返回 ErrUnsupportedEnvelopeVersion，防止协议不匹配时静默解析错字段。
		envelope.Version = events.EventVersionV1
	}
	if envelope.Version != events.EventVersionV1 {
		return envelope, fmt.Errorf("%w: version=%d", ErrUnsupportedEnvelopeVersion, envelope.Version)
	}
	if envelope.EventID == "" { // 为空时，用整条原始字节的 SHA-256 拼一个 legacy- 前缀的 ID 补上。
		/**
		为什么用哈希，而不是直接生成 UUID / 时间戳？
		
		核心诉求是同一条消息在重试、重放、重复消费时，每次解码得到的 EventID 必须完全相同。
		
		Kafka 是 at-least-once 投递，同一条消息可能被消费多次（rebalance、消费者重启、手动 seek 重放）。消费侧通常靠 EventID 做幂等去重（去重表 / 唯一索引 / 缓存）。如果这里用 uuid.New() 或时间戳，同一条原始消息每次解码都会得到不同 ID，幂等就失效了，重复消费会把业务事件写两遍。
		
		用原始字节的 SHA-256 就保证了：
		 - 确定性：相同 value → 相同哈希 → 相同 EventID，重试和重放天然幂等。
		 - 唯一性：不同业务事件的字节几乎不可能碰撞（SHA-256 抗碰撞）。
		 - 无状态：不需要查数据库、不需要生产侧配合，消费端自己就能补出来。
		
		 为什么只在 EventID == "" 时才补？
		
		注释里说得很明确：新事件必须由生产侧生成 UUID，不能靠消费端哈希兜底。原因有两个：
		- 生产侧 UUID 是业务语义上的事件唯一标识，从 Outbox 落库、投递 Kafka 到消费端全链路一致，可追溯。
		- 哈希兜底有一个理论风险：如果两条不同的合法事件恰好被序列化成完全相同的字节（比如业务字段相同、时间戳精度不够、或 Envelope 里没有区分字段），哈希会一样，被消费侧按 "同一事件" 合并掉。生产侧 UUID 从根上避免这个问题。

		*/
		// Sum256 对原始 Kafka 消息字节算 SHA-256，返回 [32]byte
		envelope.EventID = fmt.Sprintf("legacy-%x", sha256.Sum256(value))
	}
	return envelope, nil
}
