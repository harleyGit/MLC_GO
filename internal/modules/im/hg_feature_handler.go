package im

import (
	Response "MLC_GO/internal/response"
	"context"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var hgErrDisabled = errors.New("IM 能力未启用")

// HGRealtimeToken 每次签发/刷新均核实 JWT 用户与设备凭据；secret 不放入 URL。
func (h *HGHandler) HGRealtimeToken(w http.ResponseWriter, r *http.Request) {
	user, ok := hgIMUser(w, r)
	if !ok {
		return
	}
	if h.gateway == nil {
		hgIMFail(w, r, hgErrDisabled)
		return
	}
	var credential HGDeviceCredential
	if !hgIMBody(w, r, &credential) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	id, err := h.service.hgDevice(ctx, user, credential)
	if err != nil {
		hgIMFail(w, r, err)
		return
	}
	if err = h.service.HGHeartbeatDevice(ctx, user, credential); err != nil {
		hgIMFail(w, r, err)
		return
	}
	token, err := h.gateway.HGToken(hex.EncodeToString(id))
	if err != nil {
		hgIMFail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	Response.SuccessResult(w, r, map[string]any{"token": token, "url": h.websocketURL, "expiresIn": 60, "channel": "im:" + hex.EncodeToString(id)})
}

func (h *HGHandler) HGUploadInit(w http.ResponseWriter, r *http.Request) {
	user, ok := hgIMUser(w, r)
	if !ok {
		return
	}
	if h.media == nil {
		hgIMFail(w, r, hgErrDisabled)
		return
	}
	var cmd HGUploadCommand
	if !hgIMBody(w, r, &cmd) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	result, err := h.media.HGInitUpload(ctx, user, cmd)
	if err != nil {
		hgIMFail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	Response.SuccessResult(w, r, result)
}
func (h *HGHandler) HGUploadComplete(w http.ResponseWriter, r *http.Request) {
	user, ok := hgIMUser(w, r)
	if !ok {
		return
	}
	if h.media == nil {
		hgIMFail(w, r, hgErrDisabled)
		return
	}
	var cmd struct {
		MediaID string `json:"mediaId"` /* 本人上传的随机媒体 ID。 */
	}
	if !hgIMBody(w, r, &cmd) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if err := h.media.HGCompleteUpload(ctx, user, cmd.MediaID); err != nil {
		hgIMFail(w, r, err)
		return
	}
	Response.SuccessResult(w, r, map[string]bool{"accepted": true})
}
func (h *HGHandler) HGMediaStatus(w http.ResponseWriter, r *http.Request) {
	user, ok := hgIMUser(w, r)
	if !ok {
		return
	}
	if h.media == nil {
		hgIMFail(w, r, hgErrDisabled)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	result, err := h.media.HGMediaStatus(ctx, user, r.URL.Query().Get("mediaId"))
	if err != nil {
		hgIMFail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	Response.SuccessResult(w, r, result)
}

// HGMediaRead 每次 GET/Range 都鉴权和检查审核状态；流式转发私有对象，不下发可绕过授权的下载 URL。
func (h *HGHandler) HGMediaRead(w http.ResponseWriter, r *http.Request) {
	user, ok := hgIMUser(w, r)
	if !ok {
		return
	}
	if h.media == nil {
		hgIMFail(w, r, hgErrDisabled)
		return
	}
	select {
	case h.media.downloadSlots <- struct{}{}:
		defer func() { <-h.media.downloadSlots }()
	default:
		w.WriteHeader(http.StatusTooManyRequests)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	a, err := h.media.hgReadable(ctx, user, r.URL.Query().Get("mediaId"))
	if err != nil {
		hgIMFail(w, r, err)
		return
	}
	response, err := h.media.storage.HGOpen(ctx, a.sealed, r.Header.Get("Range"))
	if err != nil {
		hgIMFail(w, r, err)
		return
	}
	defer response.Body.Close()
	if response.ContentLength > a.size || (response.StatusCode == http.StatusOK && response.ContentLength != a.size) {
		hgIMFail(w, r, HGErrConflict)
		return
	}
	if response.StatusCode == http.StatusPartialContent && !strings.HasPrefix(response.Header.Get("Content-Range"), "bytes ") {
		hgIMFail(w, r, HGErrConflict)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Header().Set("Content-Type", a.mime)
	if a.kind == HGImage || a.kind == HGVideo || a.kind == HGAudio {
		w.Header().Set("Content-Disposition", "inline")
	} else {
		w.Header().Set("Content-Disposition", "attachment")
	}
	w.Header().Set("Content-Length", strconv.FormatInt(response.ContentLength, 10))
	if response.StatusCode == http.StatusPartialContent {
		w.Header().Set("Content-Range", response.Header.Get("Content-Range"))
	}
	w.Header().Set("Accept-Ranges", "bytes")
	w.WriteHeader(response.StatusCode)
	if _, err = io.CopyN(w, response.Body, response.ContentLength); err != nil {
		panic(http.ErrAbortHandler)
	}
}

func (h *HGHandler) HGMediaDelete(w http.ResponseWriter, r *http.Request) {
	user, ok := hgIMUser(w, r)
	if !ok {
		return
	}
	if h.media == nil {
		hgIMFail(w, r, hgErrDisabled)
		return
	}
	var cmd struct {
		MediaID string `json:"mediaId"` /* 本人媒体 ID。 */
	}
	if !hgIMBody(w, r, &cmd) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if err := h.media.HGDelete(ctx, user, cmd.MediaID); err != nil {
		hgIMFail(w, r, err)
		return
	}
	Response.SuccessResult(w, r, map[string]bool{"ok": true})
}

func (h *HGHandler) HGDeviceRevoke(w http.ResponseWriter, r *http.Request) {
	user, ok := hgIMUser(w, r)
	if !ok {
		return
	}
	var credential HGDeviceCredential
	if !hgIMBody(w, r, &credential) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if err := h.service.HGRevokeDevice(ctx, user, credential); err != nil {
		hgIMFail(w, r, err)
		return
	}
	Response.SuccessResult(w, r, map[string]bool{"ok": true})
}

func (h *HGHandler) HGCallCreate(w http.ResponseWriter, r *http.Request) {
	user, ok := hgIMUser(w, r)
	if !ok {
		return
	}
	if h.calls == nil {
		hgIMFail(w, r, hgErrDisabled)
		return
	}
	var cmd HGCallCreate
	if !hgIMBody(w, r, &cmd) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	result, err := h.calls.HGCreate(ctx, user, cmd)
	if err != nil {
		hgIMFail(w, r, err)
		return
	}
	Response.SuccessResult(w, r, result)
}
func (h *HGHandler) HGCallGet(w http.ResponseWriter, r *http.Request) {
	user, ok := hgIMUser(w, r)
	if !ok {
		return
	}
	if h.calls == nil {
		hgIMFail(w, r, hgErrDisabled)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	result, err := h.calls.HGGet(ctx, user, r.URL.Query().Get("callId"))
	if err != nil {
		hgIMFail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	Response.SuccessResult(w, r, result)
}
func (h *HGHandler) HGCallChange(w http.ResponseWriter, r *http.Request) {
	user, ok := hgIMUser(w, r)
	if !ok {
		return
	}
	if h.calls == nil {
		hgIMFail(w, r, hgErrDisabled)
		return
	}
	var cmd struct {
		HGDeviceCredential
		CallID string `json:"callId"` // 本人参与的呼叫。
		Action string `json:"action"` // accept/reject/cancel/end。
	}
	if !hgIMBody(w, r, &cmd) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	result, err := h.calls.HGChange(ctx, user, cmd.CallID, cmd.Action, cmd.HGDeviceCredential)
	if err != nil {
		hgIMFail(w, r, err)
		return
	}
	Response.SuccessResult(w, r, result)
}
func (h *HGHandler) HGCallToken(w http.ResponseWriter, r *http.Request) {
	user, ok := hgIMUser(w, r)
	if !ok {
		return
	}
	if h.calls == nil {
		hgIMFail(w, r, hgErrDisabled)
		return
	}
	var cmd struct {
		HGDeviceCredential
		CallID string `json:"callId"` // 已接听且绑定本设备的呼叫。
	}
	if !hgIMBody(w, r, &cmd) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	result, err := h.calls.HGToken(ctx, user, cmd.CallID, cmd.HGDeviceCredential)
	if err != nil {
		hgIMFail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	Response.SuccessResult(w, r, result)
}
