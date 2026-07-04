package order

import "MLC_GO/internal/events"

const OrderCreatedEventName = "order.created"

// OrderCreatedEvent 表示订单创建完成，后续支付、库存、风控等消费侧可独立订阅。
type OrderCreatedEvent struct {
	events.EventMeta
	// OrderID 是订单主键，也是订单事件的分区 key。
	OrderID string `json:"orderId"`
	// UserID 是下单用户，消费侧可用于用户维度风控、通知和统计。
	UserID string `json:"userId"`
}

// EventName 返回订单创建事件名称，供消费者路由。
func (e OrderCreatedEvent) EventName() string { return OrderCreatedEventName }

// EventKey 返回订单 ID，保证同一订单相关事件按 key 有序进入同一 Kafka 分区。
func (e OrderCreatedEvent) EventKey() string { return e.OrderID }
