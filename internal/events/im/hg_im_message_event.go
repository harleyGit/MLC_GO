package im

import (
	"MLC_GO/internal/events"
	"fmt"
)

const HGMessageCommittedEventName = "im.message.committed"

// HGMessageCommittedEvent 表示 MySQL 会话 seq、消息与每设备投递记录已同事务持久化；不携带私信明文。
type HGMessageCommittedEvent struct {
	events.EventMeta
	ConversationID uint64 `json:"conversationId"` // 单聊会话 ID，Kafka 同会话有序。
	MessageID      uint64 `json:"messageId"`      // 私信主键，只能经成员授权读取。
	Seq            uint64 `json:"seq"`            // 会话内单调序号。
}

func (e HGMessageCommittedEvent) EventName() string { return HGMessageCommittedEventName }
func (e HGMessageCommittedEvent) EventKey() string  { return fmt.Sprint(e.ConversationID) }

const HGCallChangedEventName = "im.call.changed"

// HGCallChangedEvent 只传播通话状态指针，不携带房间加入 Token；同会话按 Kafka key 分区。
type HGCallChangedEvent struct {
	events.EventMeta
	CallID         string `json:"callId"`         // 随机通话业务 ID。
	ConversationID uint64 `json:"conversationId"` // 所属单聊会话。
	VersionID      uint64 `json:"versionId"`      // 状态版本，处理乱序和重复事件。
}

func (e HGCallChangedEvent) EventName() string { return HGCallChangedEventName }
func (e HGCallChangedEvent) EventKey() string  { return fmt.Sprint(e.ConversationID) }
