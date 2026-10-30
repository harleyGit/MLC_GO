package im

import (
	"MLC_GO/internal/modules/im/transport"
	HGContextPackage "MLC_GO/internal/pkg/hg_context"
	HGResponsePackage "MLC_GO/internal/response"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"strconv"
)

// HGHandler 是经过现有 JWT 中间件校验的单聊 HTTP 接口；不接受客户端声明的 senderId。
type HGHandler struct {
	service      *HGService              // 独立 IM 权威服务，不复用通知或弹幕。
	gateway      *transport.HGCentrifugo // nil 时实时能力关闭。
	media        *HGPrivateMedia         // nil 时媒体能力关闭。
	calls        *HGCalls                // nil 时音视频能力关闭。
	websocketURL string                  // 受配置控制的公开网关地址。
}

func HGNewHandler(service *HGService) *HGHandler { return &HGHandler{service: service} }

// HGNewFeatureHandler 装配显式启用的 IM 外部能力，不在构造期间访问网络。
func HGNewFeatureHandler(service *HGService, gateway *transport.HGCentrifugo, media *HGPrivateMedia, calls *HGCalls, websocketURL string) *HGHandler {
	return &HGHandler{service: service, gateway: gateway, media: media, calls: calls, websocketURL: websocketURL}
}

func hgIMUser(w http.ResponseWriter, r *http.Request) (string, bool) {
	user, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePackage.FailTokenInvalid(w, r, "请先登录")
	}
	return user, ok
}

func hgIMFail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, HGErrInvalid):
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePackage.FailResult[string](w, r, HGResponsePackage.InvalidParam)
	case errors.Is(err, HGErrForbidden):
		w.WriteHeader(http.StatusForbidden)
		HGResponsePackage.FailResult[string](w, r, HGResponsePackage.InvalidParam)
	case errors.Is(err, HGErrConflict):
		w.WriteHeader(http.StatusConflict)
		HGResponsePackage.FailResult[string](w, r, HGResponsePackage.InvalidParam)
	default:
		w.WriteHeader(http.StatusServiceUnavailable)
		HGResponsePackage.FailResult[string](w, r, HGResponsePackage.DatabaseError)
	}
}

func hgIMBody(w http.ResponseWriter, r *http.Request, into any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	decoder.DisallowUnknownFields()
	if decoder.Decode(into) != nil || decoder.Decode(new(any)) != io.EOF {
		hgIMFail(w, r, HGErrInvalid)
		return false
	}
	return true
}

func hgIMID(value string) (uint64, error) {
	id, err := strconv.ParseUint(value, 10, 63)
	if err != nil || id == 0 {
		return 0, HGErrInvalid
	}
	return id, nil
}

// HGConversations 按本人 user ID 和主键游标读取单聊列表。
func (h *HGHandler) HGConversations(w http.ResponseWriter, r *http.Request) {
	user, ok := hgIMUser(w, r)
	if !ok {
		return
	}
	cursor := uint64(math.MaxInt64)
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		var err error
		cursor, err = hgIMID(raw)
		if err != nil {
			hgIMFail(w, r, err)
			return
		}
	}
	items, err := h.service.HGConversations(r.Context(), user, cursor, 30)
	if err != nil {
		hgIMFail(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, map[string]any{"list": items})
}

// HGCreateConversation 只接受对方 ID，服务端核实双方存在与唯一会话关系。
func (h *HGHandler) HGCreateConversation(w http.ResponseWriter, r *http.Request) {
	user, ok := hgIMUser(w, r)
	if !ok {
		return
	}
	var request struct {
		PeerID string `json:"peerId"`
	}
	if !hgIMBody(w, r, &request) {
		return
	}
	id, err := h.service.HGCreateConversation(r.Context(), user, request.PeerID)
	if err != nil {
		hgIMFail(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, map[string]string{"conversationId": id})
}

// HGMessages 按 seq 递增返回本人会话历史，禁止深 OFFSET 与跨用户越权。
func (h *HGHandler) HGMessages(w http.ResponseWriter, r *http.Request) {
	user, ok := hgIMUser(w, r)
	if !ok {
		return
	}
	id, err := hgIMID(r.URL.Query().Get("conversationId"))
	if err != nil {
		hgIMFail(w, r, err)
		return
	}
	after := uint64(0)
	if raw := r.URL.Query().Get("afterSeq"); raw != "" {
		after, err = strconv.ParseUint(raw, 10, 63)
		if err != nil {
			hgIMFail(w, r, HGErrInvalid)
			return
		}
	}
	items, err := h.service.HGHistory(r.Context(), user, id, after, 50)
	if err != nil {
		hgIMFail(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, map[string]any{"list": items})
}

// HGSend 校验身份和消息参数，将唯一 seq、消息、设备投递和 Outbox 原子提交。
func (h *HGHandler) HGSend(w http.ResponseWriter, r *http.Request) {
	user, ok := hgIMUser(w, r)
	if !ok {
		return
	}
	var command HGSendCommand
	if !hgIMBody(w, r, &command) {
		return
	}
	receipt, err := h.service.HGSend(r.Context(), user, command)
	if err != nil {
		hgIMFail(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, map[string]any{"message": map[string]any{
		"messageId": receipt.MessageID, "seq": receipt.Seq, "senderId": user,
		"messageType": command.MessageType, "content": command.Content, "isSelf": true,
	}})
}

// HGRead 只推进本人当前会话的合法已读 seq。
func (h *HGHandler) HGRead(w http.ResponseWriter, r *http.Request) {
	user, ok := hgIMUser(w, r)
	if !ok {
		return
	}
	var request struct {
		ConversationID string `json:"conversationId"`
		LastReadSeq    uint64 `json:"lastReadSeq"`
	}
	if !hgIMBody(w, r, &request) {
		return
	}
	id, err := hgIMID(request.ConversationID)
	if err != nil {
		hgIMFail(w, r, err)
		return
	}
	if err := h.service.HGMarkRead(r.Context(), user, id, request.LastReadSeq); err != nil {
		hgIMFail(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, map[string]bool{"ok": true})
}

// HGRegisterDevice 为已登录用户创建独立设备凭据；只返回一次，最多五台活跃设备。
func (h *HGHandler) HGRegisterDevice(w http.ResponseWriter, r *http.Request) {
	user, ok := hgIMUser(w, r)
	if !ok {
		return
	}
	credential, err := h.service.HGRegisterDevice(r.Context(), user)
	if err != nil {
		hgIMFail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	HGResponsePackage.SuccessResult(w, r, credential)
}

// HGDevicePending 仅向持有绑定设备凭据的用户返回本设备尚未确认的消息指针。
func (h *HGHandler) HGDevicePending(w http.ResponseWriter, r *http.Request) {
	user, ok := hgIMUser(w, r)
	if !ok {
		return
	}
	var request struct {
		HGDeviceCredential
		After uint64 `json:"after"`
	}
	if !hgIMBody(w, r, &request) {
		return
	}
	items, err := h.service.HGDevicePending(r.Context(), user, request.HGDeviceCredential, request.After, 100)
	if err != nil {
		hgIMFail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	HGResponsePackage.SuccessResult(w, r, map[string]any{"list": items})
}

// HGAckDevice 仅在客户端显式处理消息后记录本设备送达，不能由服务端写 socket 成功代替。
func (h *HGHandler) HGAckDevice(w http.ResponseWriter, r *http.Request) {
	user, ok := hgIMUser(w, r)
	if !ok {
		return
	}
	var request struct {
		HGDeviceCredential
		MessageID string `json:"messageId"`
	}
	if !hgIMBody(w, r, &request) {
		return
	}
	id, err := hgIMID(request.MessageID)
	if err != nil {
		hgIMFail(w, r, err)
		return
	}
	if err = h.service.HGAckDeviceMessage(r.Context(), user, request.HGDeviceCredential, id); err != nil {
		hgIMFail(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, map[string]bool{"ok": true})
}
