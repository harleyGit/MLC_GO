package video

import "MLC_GO/internal/events"

const VideoDeletedEventName = "video.deleted"

// VideoDeletedEvent 表示视频被删除或下架，消费侧据此清理读模型和索引。
type VideoDeletedEvent struct {
	events.EventMeta
	// SubmissionID 是视频投稿主键，也是该事件的 Kafka 分区 key。
	SubmissionID string `json:"submissionId"`
	// UserID 是投稿用户，消费侧可用于清理用户维度读模型。
	UserID string `json:"userId"`
}

// EventName 返回删除事件名称，供消费者路由。
func (e VideoDeletedEvent) EventName() string { return VideoDeletedEventName }

// EventKey 返回投稿 ID，保证同一投稿相关事件按 key 有序进入同一 Kafka 分区。
func (e VideoDeletedEvent) EventKey() string { return e.SubmissionID }
