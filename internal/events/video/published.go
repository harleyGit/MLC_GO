package video

import "MLC_GO/internal/events"

const VideoPublishedEventName = "video.published"

// VideoPublishedEvent 表示视频已发布，可由 Feed/Search/Statistic 消费侧构建读模型。
type VideoPublishedEvent struct {
	events.EventMeta
	// SubmissionID 是视频投稿主键，也是该事件的 Kafka 分区 key。
	SubmissionID string `json:"submissionId"`
	// UserID 是投稿用户，消费侧可用于 Feed、搜索和统计维度处理。
	UserID string `json:"userId"`
}

// EventName 返回发布事件名称，供消费者路由。
func (e VideoPublishedEvent) EventName() string { return VideoPublishedEventName }

// EventKey 返回投稿 ID，保证同一投稿相关事件按 key 有序进入同一 Kafka 分区。
func (e VideoPublishedEvent) EventKey() string { return e.SubmissionID }
