package interaction

import (
	"MLC_GO/internal/events"
	"fmt"
)

const (
	VideoInteractionChangedEventName = "video.interaction.changed"
	UserFollowChangedEventName       = "user.follow.changed"
)

// VideoInteractionChangedEvent 表示点赞、投币、收藏或分享命令已被 Kafka 接收。
type VideoInteractionChangedEvent struct {
	events.EventMeta
	ActorUserID  string `json:"actorUserId"`
	SubmissionID string `json:"submissionId"`
	Action       string `json:"action"`
	Active       bool   `json:"active,omitempty"`
	Quantity     int    `json:"quantity,omitempty"`
}

func (e VideoInteractionChangedEvent) EventName() string { return VideoInteractionChangedEventName }
func (e VideoInteractionChangedEvent) EventKey() string {
	return fmt.Sprintf("%s:%s:%s", e.SubmissionID, e.ActorUserID, e.Action)
}

// UserFollowChangedEvent 表示用户关注关系变更命令已被 Kafka 接收。
type UserFollowChangedEvent struct {
	events.EventMeta
	FollowerID string `json:"followerId"`
	FolloweeID string `json:"followeeId"`
	Active     bool   `json:"active"`
}

func (e UserFollowChangedEvent) EventName() string { return UserFollowChangedEventName }
func (e UserFollowChangedEvent) EventKey() string {
	return fmt.Sprintf("%s:%s:follow", e.FolloweeID, e.FollowerID)
}
