package im_contacts

import (
	Context "MLC_GO/internal/pkg/hg_context"
	Response "MLC_GO/internal/response"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
)

// HGHandler 只从已验证 JWT 获取调用者，禁止客户端提交 owner/userId 冒充本人。
type HGHandler struct{ service *HGService }

func HGNewHandler(db *sql.DB) *HGHandler { return &HGHandler{service: HGNewService(db)} }

func hgFail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, HGErrInvalid):
		w.WriteHeader(http.StatusBadRequest)
		Response.FailResult[string](w, r, Response.InvalidParam)
	case errors.Is(err, HGErrDenied):
		w.WriteHeader(http.StatusForbidden)
		Response.FailResult[string](w, r, Response.InvalidParam)
	case errors.Is(err, HGErrAbsent):
		w.WriteHeader(http.StatusNotFound)
		Response.FailResult[string](w, r, Response.InvalidParam)
	case errors.Is(err, HGErrState):
		w.WriteHeader(http.StatusConflict)
		Response.FailResult[string](w, r, Response.InvalidParam)
	default:
		w.WriteHeader(http.StatusServiceUnavailable)
		Response.FailResult[string](w, r, Response.DatabaseError)
	}
}

func hgBody(w http.ResponseWriter, r *http.Request, into any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	d.DisallowUnknownFields()
	if d.Decode(into) != nil || d.Decode(new(any)) != io.EOF {
		hgFail(w, r, HGErrInvalid)
		return false
	}
	return true
}

func hgViewer(w http.ResponseWriter, r *http.Request) (string, bool) {
	id, ok := Context.CurrentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		Response.FailTokenInvalid(w, r, "请先登录")
	}
	return id, ok
}

func hgAfter(r *http.Request) (uint64, error) {
	raw := r.URL.Query().Get("after")
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.ParseUint(raw, 10, 63)
	if err != nil {
		return 0, HGErrInvalid
	}
	return n, nil
}

// HGFriends 返回本人好友第一页；前端通过 after 游标续页。
func (h *HGHandler) HGFriends(w http.ResponseWriter, r *http.Request) {
	user, ok := hgViewer(w, r)
	if !ok {
		return
	}
	after, err := hgAfter(r)
	if err != nil {
		hgFail(w, r, err)
		return
	}
	items, err := h.service.HGList(r.Context(), user, after)
	if err != nil {
		hgFail(w, r, err)
		return
	}
	Response.SuccessResult(w, r, map[string]any{"list": items})
}

// HGIncoming 仅返回本人收到的待处理申请。
func (h *HGHandler) HGIncoming(w http.ResponseWriter, r *http.Request) {
	user, ok := hgViewer(w, r)
	if !ok {
		return
	}
	after, err := hgAfter(r)
	if err != nil {
		hgFail(w, r, err)
		return
	}
	items, err := h.service.HGIncoming(r.Context(), user, after)
	if err != nil {
		hgFail(w, r, err)
		return
	}
	Response.SuccessResult(w, r, map[string]any{"list": items})
}

// HGBlocks 仅返回本人主动拉黑的用户。
func (h *HGHandler) HGBlocks(w http.ResponseWriter, r *http.Request) {
	user, ok := hgViewer(w, r)
	if !ok {
		return
	}
	after, err := hgAfter(r)
	if err != nil {
		hgFail(w, r, err)
		return
	}
	items, err := h.service.HGBlocks(r.Context(), user, after)
	if err != nil {
		hgFail(w, r, err)
		return
	}
	Response.SuccessResult(w, r, map[string]any{"list": items})
}

// HGProfile 返回指定用户的公开资料，始终不返回其手机号和邮箱。
func (h *HGHandler) HGProfile(w http.ResponseWriter, r *http.Request) {
	if _, ok := hgViewer(w, r); !ok {
		return
	}
	id := r.URL.Query().Get("userId")
	if !hgValidID(id) {
		hgFail(w, r, HGErrInvalid)
		return
	}
	item, err := h.service.HGProfileFor(r.Context(), id)
	if err != nil {
		hgFail(w, r, err)
		return
	}
	Response.SuccessResult(w, r, item)
}

// HGSearch 仅支持精确的公开 ID 和自定义 ID；不开放隐私身份检索。
func (h *HGHandler) HGSearch(w http.ResponseWriter, r *http.Request) {
	viewer, ok := hgViewer(w, r)
	if !ok {
		return
	}
	var req struct {
		Type    string `json:"type"`
		Keyword string `json:"keyword"`
	}
	if !hgBody(w, r, &req) {
		return
	}
	item, err := h.service.HGSearch(r.Context(), viewer, req.Type, req.Keyword)
	if err != nil {
		hgFail(w, r, err)
		return
	}
	Response.SuccessResult(w, r, item)
}

// HGSend 发送申请或处理反向申请；本人身份只能来自 JWT。
func (h *HGHandler) HGSend(w http.ResponseWriter, r *http.Request) {
	user, ok := hgViewer(w, r)
	if !ok {
		return
	}
	var req struct {
		TargetUserID string `json:"targetUserId"`
		Message      string `json:"message"`
	}
	if !hgBody(w, r, &req) {
		return
	}
	if err := h.service.HGSendRequest(r.Context(), user, req.TargetUserID, req.Message); err != nil {
		hgFail(w, r, err)
		return
	}
	Response.SuccessResult(w, r, map[string]bool{"ok": true})
}

// HGDecision 对本人收到的申请作接受或拒绝决定，客户端不得指定申请人身份。
func (h *HGHandler) HGDecision(accept bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := hgViewer(w, r)
		if !ok {
			return
		}
		var req struct {
			RequestID string `json:"requestId"`
		}
		if !hgBody(w, r, &req) {
			return
		}
		item, err := h.service.HGDecide(r.Context(), user, req.RequestID, accept)
		if err != nil {
			hgFail(w, r, err)
			return
		}
		Response.SuccessResult(w, r, item)
	}
}

// HGEdit 只修改本人给好友的私有备注字段。
func (h *HGHandler) HGEdit(w http.ResponseWriter, r *http.Request) {
	user, ok := hgViewer(w, r)
	if !ok {
		return
	}
	var req struct {
		UserID string `json:"userId"`
		HGPatch
	}
	if !hgBody(w, r, &req) {
		return
	}
	if err := h.service.HGEdit(r.Context(), user, req.UserID, req.HGPatch); err != nil {
		hgFail(w, r, err)
		return
	}
	Response.SuccessResult(w, r, map[string]bool{"ok": true})
}

// HGAction 管理当前用户的好友关系和有方向黑名单；不删除历史消息。
func (h *HGHandler) HGAction(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := hgViewer(w, r)
		if !ok {
			return
		}
		var req struct {
			UserID string `json:"userId"`
		}
		if !hgBody(w, r, &req) {
			return
		}
		var err error
		switch action {
		case "remove":
			err = h.service.HGRemove(r.Context(), user, req.UserID)
		case "block":
			err = h.service.HGSetBlock(r.Context(), user, req.UserID, true)
		case "unblock":
			err = h.service.HGSetBlock(r.Context(), user, req.UserID, false)
		}
		if err != nil {
			hgFail(w, r, err)
			return
		}
		Response.SuccessResult(w, r, map[string]bool{"ok": true})
	}
}
