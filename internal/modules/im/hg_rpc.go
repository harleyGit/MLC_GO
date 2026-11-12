package im

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

// HGRPCHandler 只处理 Centrifugo 管理面代理请求；连接用户与设备凭据必须双重匹配。
type HGRPCHandler struct {
	service *HGService      // 复用 MySQL/Outbox 权威写入与成员校验。
	secret  []byte          // 独立代理密钥，禁止转发客户端同名请求头覆盖。
	media   *HGPrivateMedia // 私有下载需逐段重新授权，nil 表示未启用。
}

// HGNewRPCHandler 在独立管理面装配代理；不得将此 handler 暴露到业务 API 或未隔离的监听地址。
func HGNewRPCHandler(service *HGService, media *HGPrivateMedia, secret string) http.Handler {
	return &HGRPCHandler{service: service, media: media, secret: []byte(secret)}
}

// HGRPCRequest 是 Centrifugo v6 RPC proxy JSON 协议；data 仅承载小体积消息/游标，绝不承载附件字节。
type HGRPCRequest struct {
	User      string          `json:"user"`      // 已认证连接 subject，格式 im/<deviceId>；必须由可信代理传递。
	Transport string          `json:"transport"` // 仅接收 websocket，拒绝其他可用传输。
	Method    string          `json:"method"`    // 只允许 im:send / im:history / im:pending / im:ack / im:media_read。
	Data      json.RawMessage `json:"data"`      // 含本设备凭据的有界请求，不能作为权威用户身份。
}

// HGRPCData 为所有 IM RPC 共用的设备绑定字段和有界业务参数。
type HGRPCData struct {
	HGDeviceCredential
	ConversationID string          `json:"conversationId"` // 本人会话主键；history/send 使用。
	ClientMsgID    string          `json:"clientMsgId"`    // 用户级幂等 ID；send 使用。
	MessageType    HGMessageType   `json:"messageType"`    // 发送消息类型；send 使用。
	Content        json.RawMessage `json:"content"`        // 最多 4096 字节的文本或媒体引用。
	AfterSeq       uint64          `json:"afterSeq"`       // 会话 seq 游标；history 使用。
	After          uint64          `json:"after"`          // 设备消息主键游标；pending 使用。
	MessageID      string          `json:"messageId"`      // 消息主键；ack 使用。
	MediaID        string          `json:"mediaId"`        // 私有已审核附件 ID；media_read 使用。
}

// HGRPCError 使用 Centrifugo RPC proxy 自定义错误格式；code 不携带内部错误文本。
type HGRPCError struct {
	Code    int    `json:"code"`    // 400 参数无效，403 凭据/权限无效，409 冲突，503 暂不可用。
	Message string `json:"message"` // 可展示的固定中文错误，不回显用户输入或凭据。
}

func hgRPCFail(w http.ResponseWriter, err error) {
	code, msg := 503, "服务暂不可用"
	switch {
	case errors.Is(err, HGErrInvalid):
		code, msg = 400, "请求参数无效"
	case errors.Is(err, HGErrForbidden):
		code, msg = 403, "设备或会话权限无效"
	case errors.Is(err, HGErrConflict):
		code, msg = 409, "消息状态冲突"
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"error": HGRPCError{Code: code, Message: msg}})
}

// ServeHTTP 先校验代理密钥，再独立校验当前设备归属和 secret；RPC 结果只在事务提交成功后返回。
func (h *HGRPCHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost || len(h.secret) < 32 || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-IM-RPC-Secret")), h.secret) != 1 {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	var request HGRPCRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	if decoder.Decode(&request) != nil || decoder.Decode(new(any)) != io.EOF || request.Transport != "websocket" || !strings.HasPrefix(request.User, "im/") {
		hgRPCFail(w, HGErrInvalid)
		return
	}
	device := strings.TrimPrefix(request.User, "im/")
	if len(device) != 32 || len(request.Data) == 0 || len(request.Data) > 6144 {
		hgRPCFail(w, HGErrInvalid)
		return
	}
	var data HGRPCData
	if json.Unmarshal(request.Data, &data) != nil || data.DeviceID != device {
		hgRPCFail(w, HGErrForbidden)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	user, err := h.hgUser(ctx, data.HGDeviceCredential)
	if err != nil {
		hgRPCFail(w, err)
		return
	}
	var result any
	switch request.Method {
	case "im:send":
		command := HGSendCommand{ConversationID: data.ConversationID, ClientMsgID: data.ClientMsgID, MessageType: data.MessageType, Content: data.Content}
		var receipt HGSendReceipt
		receipt, err = h.service.HGSend(ctx, user, command)
		if err == nil {
			result = map[string]any{"message": map[string]any{"messageId": receipt.MessageID, "seq": receipt.Seq, "senderId": user, "messageType": command.MessageType, "content": command.Content, "isSelf": true}}
		}
	case "im:history":
		var id uint64
		id, err = hgIMID(data.ConversationID)
		if err == nil && data.AfterSeq > uint64(^uint64(0)>>1) {
			err = HGErrInvalid
		}
		if err == nil {
			var items any
			items, err = h.service.HGHistory(ctx, user, id, data.AfterSeq, 50)
			if err == nil {
				var floor uint64
				floor, err = h.service.HGHistoryFloor(ctx, id)
				result = map[string]any{"list": items, "floorSeq": floor}
			}
		}
	case "im:pending":
		var items []HGDeviceDelivery
		items, err = h.service.HGDevicePending(ctx, user, data.HGDeviceCredential, data.After, 100)
		result = map[string]any{"list": items}
	case "im:ack":
		var id uint64
		id, err = hgIMID(data.MessageID)
		if err == nil {
			err = h.service.HGAckDeviceMessage(ctx, user, data.HGDeviceCredential, id)
		}
		result = map[string]bool{"ok": true}
	case "im:media_read":
		if h.media == nil {
			err = HGErrForbidden
			break
		}
		var url string
		url, err = h.media.HGReadGrant(ctx, user, data.MediaID)
		result = map[string]any{"url": url, "expiresIn": 15}
	default:
		err = HGErrInvalid
	}
	if err != nil {
		hgRPCFail(w, err)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"data": result}})
}

// hgUser 以设备主键索引确定用户后核实 secret 和撤销状态，不采信 RPC 自报的 userId。
func (h *HGRPCHandler) hgUser(ctx context.Context, credential HGDeviceCredential) (string, error) {
	if h.service == nil || h.service.db == nil || len(credential.DeviceID) != 32 {
		return "", HGErrForbidden
	}
	id, err := hex.DecodeString(credential.DeviceID)
	if err != nil || len(id) != 16 {
		return "", HGErrForbidden
	}
	var user string
	if err := h.service.db.QueryRowContext(ctx, SQLQueriesPackage.HGIMRPCDeviceOwnerSQL, id).Scan(&user); err != nil {
		return "", HGErrForbidden
	}
	if _, err = h.service.hgDevice(ctx, user, credential); err != nil {
		return "", HGErrForbidden
	}
	return user, nil
}
