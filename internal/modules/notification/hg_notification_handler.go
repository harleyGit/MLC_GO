package notification

import (
	HGContextPackage "MLC_GO/internal/pkg/hg_context"
	HGResponsePackage "MLC_GO/internal/response"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
)

// HGHandler 将已认证用户的收件箱操作映射到有界数据库请求。
type HGHandler struct{ service *HGService }

// HGNewHandler 创建通知 HTTP 入口。
func HGNewHandler(service *HGService) *HGHandler { return &HGHandler{service: service} }

func hgCursor(raw string) (uint64, error) {
	if raw == "" {
		// database/sql 的 MySQL 驱动不接受超过 MaxInt64 的 uint64 参数。
		return math.MaxInt64, nil
	}
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || value == 0 || value > math.MaxInt64 {
		return 0, HGErrInvalid
	}
	return value, nil
}
func hgPageSize(raw string) (int, error) {
	if raw == "" {
		return 20, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 || value > 50 {
		return 0, HGErrInvalid
	}
	return value, nil
}
func hgUser(w http.ResponseWriter, r *http.Request) (string, bool) {
	user, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePackage.FailTokenInvalid(w, r, "登录状态无效")
	}
	return user, ok
}
func hgFail(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, HGErrInvalid) {
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePackage.FailResult[string](w, r, HGResponsePackage.InvalidParam)
		return
	}
	w.WriteHeader(http.StatusServiceUnavailable)
	HGResponsePackage.FailResult[string](w, r, HGResponsePackage.DatabaseError)
}
func hgBody(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if decoder.Decode(target) != nil || decoder.Decode(new(any)) != io.EOF {
		hgFail(w, r, HGErrInvalid)
		return false
	}
	return true
}

// HGList 查询当前用户单分类的有界通知页。
func (handler *HGHandler) HGList(w http.ResponseWriter, r *http.Request) {
	user, ok := hgUser(w, r)
	if !ok {
		return
	}
	category := r.URL.Query().Get("category")
	if !hgCategories[category] {
		hgFail(w, r, HGErrInvalid)
		return
	}
	cursor, err := hgCursor(r.URL.Query().Get("cursor"))
	if err != nil {
		hgFail(w, r, err)
		return
	}
	size, err := hgPageSize(r.URL.Query().Get("pageSize"))
	if err != nil {
		hgFail(w, r, err)
		return
	}
	unread := r.URL.Query().Get("unreadOnly") == "1"
	items, next, err := handler.service.repo.HGPage(r.Context(), user, category, cursor, size, unread)
	if err != nil {
		hgFail(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, map[string]any{"list": items, "nextCursor": next})
}

// HGUnread 返回本人按分类的权威未读数。
func (handler *HGHandler) HGUnread(w http.ResponseWriter, r *http.Request) {
	user, ok := hgUser(w, r)
	if !ok {
		return
	}
	items, err := handler.service.repo.HGUnread(r.Context(), user)
	if err != nil {
		hgFail(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, map[string]any{"unreadByCategory": items})
}

// HGRead 批量已读单次最多 50 条；全部已读单次最多 500 条并回传 hasMore。
func (handler *HGHandler) HGRead(w http.ResponseWriter, r *http.Request) {
	user, ok := hgUser(w, r)
	if !ok {
		return
	}
	var request struct {
		NotificationIDs []string `json:"notificationIds"`
		Category        string   `json:"category"`
		All             bool     `json:"all"`
	}
	if !hgBody(w, r, &request) {
		return
	}
	if request.All {
		if !hgCategories[request.Category] || len(request.NotificationIDs) > 0 {
			hgFail(w, r, HGErrInvalid)
			return
		}
		more, err := handler.service.repo.HGReadBatch(r.Context(), user, request.Category)
		if err != nil {
			hgFail(w, r, err)
			return
		}
		HGResponsePackage.SuccessResult(w, r, map[string]bool{"hasMore": more})
		return
	}
	if len(request.NotificationIDs) < 1 || len(request.NotificationIDs) > 50 || request.Category != "" {
		hgFail(w, r, HGErrInvalid)
		return
	}
	ids := make([]uint64, 0, len(request.NotificationIDs))
	for _, raw := range request.NotificationIDs {
		id, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || id == 0 || id > math.MaxInt64 {
			hgFail(w, r, HGErrInvalid)
			return
		}
		ids = append(ids, id)
	}
	if err := handler.service.repo.HGReadIDs(r.Context(), user, ids); err != nil {
		hgFail(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, map[string]bool{"ok": true})
}

// HGDelete 软删除当前用户收件箱中的一条通知。
func (handler *HGHandler) HGDelete(w http.ResponseWriter, r *http.Request) {
	user, ok := hgUser(w, r)
	if !ok {
		return
	}
	var request struct {
		NotificationID string `json:"notificationId"`
	}
	if !hgBody(w, r, &request) {
		return
	}
	id, err := strconv.ParseUint(request.NotificationID, 10, 64)
	if err != nil || id == 0 || id > math.MaxInt64 {
		hgFail(w, r, HGErrInvalid)
		return
	}
	if err = handler.service.repo.HGDelete(r.Context(), user, id); err != nil {
		hgFail(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, map[string]bool{"ok": true})
}

// HGGetPreferences 返回本人的通知偏好。
func (handler *HGHandler) HGGetPreferences(w http.ResponseWriter, r *http.Request) {
	user, ok := hgUser(w, r)
	if !ok {
		return
	}
	result, err := handler.service.repo.HGPreferences(r.Context(), user)
	if err != nil {
		hgFail(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, result)
}

// HGSetPreferences 校验偏好后仅更新本人的设置。
func (handler *HGHandler) HGSetPreferences(w http.ResponseWriter, r *http.Request) {
	user, ok := hgUser(w, r)
	if !ok {
		return
	}
	var pref HGPreferences
	if !hgBody(w, r, &pref) {
		return
	}
	if err := handler.service.HGSavePreferences(r.Context(), user, pref); err != nil {
		hgFail(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, map[string]bool{"ok": true})
}

// HGChannels 只报告站内信为可用，未接供应商渠道不可操作也不产生伪指标。
func (handler *HGHandler) HGChannels(w http.ResponseWriter, r *http.Request) {
	channels := []map[string]any{}
	for _, entry := range []struct {
		code, name string
		ready      bool
	}{{"in_app", "站内信", true}, {"sms", "短信", false}, {"email", "邮件", false}, {"push", "移动推送", false}, {"realtime", "WebSocket 提醒", false}} {
		channels = append(channels, map[string]any{"code": entry.code, "name": entry.name, "desc": map[bool]string{true: "MySQL 持久化站内通知", false: "尚未接入供应商/实时网关"}[entry.ready], "enabled": entry.ready, "available": entry.ready, "providers": []string{}, "dailySent": 0, "successRate": 0, "p95": "--", "tone": "blue", "primaryMetricLabel": "投递量", "config": map[string]any{"priority": "normal", "rateLimitQps": 1, "dailyQuota": 0}})
	}
	HGResponsePackage.SuccessResult(w, r, map[string]any{"channels": channels, "providers": []any{}})
}

// HGDeliveries 只允许查看已落库的站内通知，不冒充短信/邮件投递状态。
func (handler *HGHandler) HGDeliveries(w http.ResponseWriter, r *http.Request) {
	cursor, err := hgCursor(r.URL.Query().Get("cursor"))
	if err != nil {
		hgFail(w, r, err)
		return
	}
	size, err := hgPageSize(r.URL.Query().Get("pageSize"))
	if err != nil {
		hgFail(w, r, err)
		return
	}
	items, next, err := handler.service.repo.HGDeliveries(r.Context(), cursor, size)
	if err != nil {
		hgFail(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, map[string]any{"list": items, "nextCursor": next})
}

// HGTestSend 只向 JWT 管理员本人写入系统站内信，不调用任何外部供应商。
func (handler *HGHandler) HGTestSend(w http.ResponseWriter, r *http.Request) {
	user, ok := hgUser(w, r)
	if !ok {
		return
	}
	var request struct {
		Code string `json:"code"`
	}
	if !hgBody(w, r, &request) {
		return
	}
	if request.Code != "in_app" {
		hgFail(w, r, HGErrInvalid)
		return
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		hgFail(w, r, err)
		return
	}
	if err := handler.service.HGDeliver(r.Context(), fmt.Sprintf("notify-test-%x", nonce), user, HGNotice{Category: "system", Title: "站内通知试发送", Content: "站内信已成功写入消息中心。"}); err != nil {
		hgFail(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, map[string]bool{"ok": true})
}
