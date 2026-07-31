package coin

import (
	"MLC_GO/internal/events"
	"fmt"
)

// HGAssetChangedEventName 是硬币资产变化的稳定协议名；消费者应依据事件版本演进字段，不能按 Go 类型名耦合。
const HGAssetChangedEventName = "coin.asset.changed"

// HGAssetChangedEvent 描述已在 MySQL 权威事务中提交的硬币资产变化。
//
// 事件通过同事务 Outbox 发布，因此收到事件只代表资产变化已经持久化，消费方不得再次修改权威余额。
// EventKey 固定按用户分区，保证同一用户的充值、赠币、扣减、过期和退款事件保持 Kafka 顺序。
type HGAssetChangedEvent struct {
	events.EventMeta
	UserID                 string `json:"userId"`
	Operation              string `json:"operation"`
	Amount                 uint64 `json:"amount"`
	BalanceAfter           uint64 `json:"balanceAfter,omitempty"`
	BusinessType           string `json:"businessType,omitempty"`
	BusinessKey            string `json:"businessKey,omitempty"`
	ReferenceTransactionID uint64 `json:"referenceTransactionId,omitempty"`
}

// EventName 返回跨服务稳定的领域事件名。
func (e HGAssetChangedEvent) EventName() string { return HGAssetChangedEventName }

// EventKey 将同一用户的资产事件固定到同一 Kafka partition。
func (e HGAssetChangedEvent) EventKey() string { return fmt.Sprintf("%s:coin", e.UserID) }
