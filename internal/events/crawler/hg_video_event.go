// Package crawler 定义外部视频采集流水线 v1 消息协议。
package crawler

import (
	"MLC_GO/internal/events"
	"encoding/json"
	"github.com/google/uuid"
	"time"
)

const (
	HGDiscovered  = "crawler.video.discovered.v1"
	HGRaw         = "crawler.video.raw.v1"
	HGCleaned     = "crawler.video.cleaned.v1"
	HGStored      = "crawler.video.stored.v1"
	HGTopicPrefix = "mlc."
)

// HGVideoEvent 是阶段消息载荷；Data 仅含详情响应或清洗快照，不携带 Cookie。
type HGVideoEvent struct {
	BVID         string          `json:"bvid"`           // 视频标识，与信封 key 一致。
	CollectionID string          `json:"collectionId"`   // 一次发现的稳定 UUID，各阶段保持不变。
	CollectedAt  time.Time       `json:"collectedAt"`    // 实际抓取时间；发现阶段为空。
	Data         json.RawMessage `json:"data,omitempty"` // 原始或清洗 JSON，由消费者限制大小。
}

// HGEnvelope 构造固定 ID 的阶段消息，同次采集重放不得生成新的幂等身份。
func HGEnvelope(stage string, item HGVideoEvent) (events.EventEnvelope, error) {
	payload, err := json.Marshal(item)
	timestamp := item.CollectedAt.UnixMilli()
	if item.CollectedAt.IsZero() {
		timestamp = time.Now().UTC().UnixMilli()
	}
	return events.EventEnvelope{EventID: HGEventID(item.CollectionID, stage), EventName: stage,
		EventKey: "bilibili:" + item.BVID, Version: 1, SourceService: events.SourceServiceMLC,
		Timestamp: timestamp, Payload: payload}, err
}

// HGEventID 使用稳定UUID承载阶段身份，满足既有Outbox event_id长度限制。
func HGEventID(collectionID, stage string) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(collectionID+":"+stage)).String()
}
