package im

import (
	"MLC_GO/internal/modules/im/transport"
	UserJWTMiddlewarePackage "MLC_GO/internal/modules/user/middleware"
	UserServicePackage "MLC_GO/internal/modules/user/service"
	HGContextPackage "MLC_GO/internal/pkg/hg_context"
	HGResponsePackage "MLC_GO/internal/response"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"strconv"
	"time"
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
	numericID, _ := hgIMID(id)
	item, err := h.service.HGConversationByID(r.Context(), user, numericID)
	if err != nil {
		hgIMFail(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, map[string]any{"conversationId": id, "conversation": item})
}

// HGConversationMuted 仅变更本人会话免打扰标志，不改变设备投递或消息接收。
func (h *HGHandler) HGConversationMuted(w http.ResponseWriter, r *http.Request) {
	user, ok := hgIMUser(w, r)
	if !ok {
		return
	}
	var request struct {
		ConversationID string `json:"conversationId"`
		Muted          *bool  `json:"muted"`
	}
	if !hgIMBody(w, r, &request) {
		return
	}
	id, err := hgIMID(request.ConversationID)
	if err != nil || request.Muted == nil {
		hgIMFail(w, r, HGErrInvalid)
		return
	}
	if err := h.service.HGSetConversationMuted(r.Context(), user, id, *request.Muted); err != nil {
		hgIMFail(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, map[string]bool{"ok": true})
}

// HGDeleteConversation 分批物理删除三天以前的双方历史，仅完成后隐藏本人会话；客户端用 baseSeq 重试。
func (h *HGHandler) HGDeleteConversation(w http.ResponseWriter, r *http.Request) {
	user, ok := hgIMUser(w, r)
	if !ok {
		return
	}
	var request struct {
		ConversationID string `json:"conversationId"`
		BaseSeq        uint64 `json:"baseSeq"`
	}
	if !hgIMBody(w, r, &request) {
		return
	}
	id, err := hgIMID(request.ConversationID)
	if err != nil {
		hgIMFail(w, r, err)
		return
	}
	baseSeq, more, err := h.service.HGDeleteConversationBatch(r.Context(), user, id, request.BaseSeq)
	if err != nil {
		hgIMFail(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, map[string]any{"baseSeq": baseSeq, "hasMore": more})
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
	claims, _ := r.Context().Value(UserJWTMiddlewarePackage.UserIDKey).(*UserServicePackage.HGClaims)
	var credential HGDeviceCredential
	var err error
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if claims != nil && claims.SessionID != "" {
		credential, err = h.service.HGRegisterSessionDevice(ctx, user, claims.SessionID)
	} else {
		credential, err = h.service.HGRegisterDevice(ctx, user) // 发布前的旧 token 兼容注册，但不能自动踢旧会话。
	}
	if err != nil {
		hgIMFail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if credential.LegacyReplaced {
		HGResponsePackage.SuccessResult(w, r, map[string]any{"deviceId": credential.DeviceID, "secret": credential.Secret, "legacyReplaced": true})
		return
	}
	HGResponsePackage.SuccessResult(w, r, credential)
}
