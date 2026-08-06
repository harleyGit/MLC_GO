package video_danmaku

import "MLC_GO/internal/events"

const VideoDanmakuCreatedEventName = "video.danmaku.created"

// CreatedEvent 是 MySQL 热数据提交后投递到 Kafka 的稳定弹幕历史事件。
type CreatedEvent struct {
	events.EventMeta
	DanmakuID    string `json:"danmakuId"`
	SubmissionID string `json:"submissionId"`
	VideoID      string `json:"videoId"`
	UserID       string `json:"userId"`
	RequestID    string `json:"requestId"`
	Content      string `json:"content"`
	ProgressMS   uint32 `json:"progressMs"`
	Mode         string `json:"mode"`
	Color        string `json:"color"`
	FontSize     uint8  `json:"fontSize"`
	CreatedAt    int64  `json:"createdAt"`
}

func (e CreatedEvent) EventName() string { return VideoDanmakuCreatedEventName }
func (e CreatedEvent) EventKey() string  { return e.VideoID }
