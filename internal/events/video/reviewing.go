package video

import "MLC_GO/internal/events"

const VideoReviewedEventName = "video.reviewed"

// VideoReviewedEvent 表示稿件已提交审核或进入审核流。
// 不直接传数据库 Model，避免把库字段、内部状态和跨服务字节协议强耦合。
type VideoReviewedEvent struct {
	events.EventMeta
	// SubmissionID 是视频投稿主键，也是该事件的 Kafka 分区 key。
	SubmissionID string `json:"submissionId"`
	// UserID 是投稿用户，消费侧可用于用户维度统计或通知。
	UserID string `json:"userId"`
}

// EventName 返回审核事件名称，供消费者路由。
func (e VideoReviewedEvent) EventName() string { return VideoReviewedEventName }

// EventKey 返回投稿 ID，保证同一投稿相关事件按 key 有序进入同一 Kafka 分区。
func (e VideoReviewedEvent) EventKey() string { return e.SubmissionID }
