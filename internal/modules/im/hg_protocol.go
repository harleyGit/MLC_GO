// Package im 提供独立消息权威存储、设备投递、私有附件与通话业务；不复用弹幕广播。
package im

import (
	"context"
	"encoding/json"
)

// HGMessageType 是未来私信消息内容类型；附件仅携带经授权的 mediaId，不传 Base64 或公开 URL。
type HGMessageType string

const (
	HGText  HGMessageType = "text"  // 纯文本私信。
	HGImage HGMessageType = "image" // 已审核的私有图片引用。
	HGVideo HGMessageType = "video" // 已审核的私有视频引用。
	HGAudio HGMessageType = "audio" // 已审核的私有音频引用。
	HGFile  HGMessageType = "file"  // 已审核的私有文件引用。
)

// HGSendCommand 是未来 IM 发送协议；sender 必须由可信 JWT 身份注入，不在请求体中声明。
type HGSendCommand struct {
	ConversationID string          `json:"conversationId"` // 受服务端成员权限保护的会话 ID。
	ClientMsgID    string          `json:"clientMsgId"`    // 用户级幂等键；重试须校验内容一致。
	MessageType    HGMessageType   `json:"messageType"`    // text/image/video 类型枚举。
	Content        json.RawMessage `json:"content"`        // 小体积 JSON 元数据；实际媒体保存在私有对象存储。
}

// HGMediaContent 仅保存媒体引用；宽高/时长由上传审核链路校验并填充，不信任客户端。
type HGMediaContent struct {
	MediaID    string `json:"mediaId"`    // 已审核且归当前发送者所有的媒体 ID。
	Width      int    `json:"width"`      // 媒体像素宽度；视频/图片适用。
	Height     int    `json:"height"`     // 媒体像素高度；视频/图片适用。
	DurationMS int64  `json:"durationMs"` // 视频时长毫秒；图片为 0。
}

// HGMessageService 将发送与存储顺序、Kafka 投递和客户端展示解耦；未来实现须先持久化才返回 ACK。
type HGMessageService interface {
	HGSend(context.Context, string, HGSendCommand) (HGSendReceipt, error)
}

// HGSendReceipt 仅表示服务端已持久化，不代表接收端送达或已读。
type HGSendReceipt struct {
	MessageID string `json:"messageId"` // 服务端唯一消息 ID。
	Seq       uint64 `json:"seq"`       // 服务端分配的会话内单调序列号。
}

// HGMediaAuthorizer 是未来私有媒体授权边界；上传/读媒体必须同时校验归属、类型和审核状态。
type HGMediaAuthorizer interface {
	HGAuthorizeSend(context.Context, string, string, HGMessageType) error
}
