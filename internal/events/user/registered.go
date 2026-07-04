package user

import "MLC_GO/internal/events"

const UserRegisteredEventName = "user.registered"

// UserRegisteredEvent 表示用户注册完成，用于积分、消息、统计等异步消费者。
type UserRegisteredEvent struct {
	events.EventMeta
	// UserID 是注册完成的用户 ID，也是用户事件的分区 key。
	UserID string `json:"userId"`
}

// EventName 返回用户注册事件名称，供消费者路由。
func (e UserRegisteredEvent) EventName() string { return UserRegisteredEventName }

// EventKey 返回用户 ID，保证同一用户相关事件按 key 有序进入同一 Kafka 分区。
func (e UserRegisteredEvent) EventKey() string { return e.UserID }
