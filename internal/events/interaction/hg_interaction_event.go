/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-07-31 12:46:59
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-10-16 15:52:45
 * @FilePath: /MLC_GO/internal/events/interaction/hg_interaction_event.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package interaction

import (
	"MLC_GO/internal/events"
	"fmt"
)

const (

	// VideoInteractionChangedEventName 视频互动变化事件，例如点赞、取消点赞
	VideoInteractionChangedEventName = "video.interaction.changed"

	// UserFollowChangedEventName  用户关注变化事件，例如关注、取消关注
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
	FollowID   string `json:"followId"`
	FollowerID string `json:"followerId"`
	FolloweeID string `json:"followeeId"`
	Active     bool   `json:"active"`
}

func (e UserFollowChangedEvent) EventName() string { return UserFollowChangedEventName }
func (e UserFollowChangedEvent) EventKey() string {
	return fmt.Sprintf("%s:%s:follow", e.FolloweeID, e.FollowerID)
}
