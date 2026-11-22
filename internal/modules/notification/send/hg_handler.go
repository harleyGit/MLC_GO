package send

import (
	contextpkg "MLC_GO/internal/pkg/hg_context"
	response "MLC_GO/internal/response"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"strconv"
)

// HGHandler 将 IAM 路由的管理请求映射到双人发布及有界任务，身份始终取可信 JWT。
type HGHandler struct {
	repo *HGRepository // 持久任务与发布事务。
}

// HGNewHandler 使用共享 MySQL 仓储，不对外暴露内部工作队列。
func HGNewHandler(repo *HGRepository) *HGHandler { return &HGHandler{repo: repo} }

func hgBody(w http.ResponseWriter, r *http.Request, value any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384))
	decoder.DisallowUnknownFields()
	if decoder.Decode(value) != nil || decoder.Decode(new(any)) != io.EOF {
		return HGErrInvalid
	}
	return nil
}

func hgFail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, HGErrInvalid):
		w.WriteHeader(http.StatusBadRequest)
	case errors.Is(err, HGErrConflict):
		w.WriteHeader(http.StatusConflict)
	case errors.Is(err, HGErrMissing):
		w.WriteHeader(http.StatusNotFound)
	case errors.Is(err, HGErrBusy):
		w.WriteHeader(http.StatusTooManyRequests)
		response.FailResult[string](w, r, response.RateLimit)
		return
	default:
		w.WriteHeader(http.StatusServiceUnavailable)
		response.FailResult[string](w, r, response.DatabaseError)
		return
	}
	response.FailResult[string](w, r, response.InvalidParam)
}

func hgActor(r *http.Request) (string, error) {
	user, ok := contextpkg.CurrentUserID(r)
	if !ok || !hgUserID.MatchString(user) {
		return "", HGErrInvalid
	}
	return user, nil
}

// HGPublish 发布经过另一位管理员复核的当前站内模板版本。
func (handler *HGHandler) HGPublish(w http.ResponseWriter, r *http.Request) {
	user, err := hgActor(r)
	id, idErr := hgID(r.URL.Query().Get("id"))
	if err == nil {
		err = idErr
	}
	var request struct {
		Version uint32 `json:"version"` // 期望发布的最新版本号，不能为零。
	}
	if err == nil {
		err = hgBody(w, r, &request)
	}
	if err == nil && request.Version == 0 {
		err = HGErrInvalid
	}
	if err == nil {
		err = handler.repo.HGPublish(r.Context(), id, request.Version, user)
	}
	if err != nil {
		hgFail(w, r, err)
		return
	}
	response.SuccessResult(w, r, map[string]any{"id": r.URL.Query().Get("id"), "publishedVersion": request.Version, "status": "published"})
}

// HGCreate 创建异步持久化任务；HTTP 不等待实际投递，不接受客户端声称的操作者。
func (handler *HGHandler) HGCreate(w http.ResponseWriter, r *http.Request) {
	user, err := hgActor(r)
	var cmd HGCommand
	if err == nil {
		err = hgBody(w, r, &cmd)
	}
	var id string
	if err == nil {
		id, err = handler.repo.HGCreate(r.Context(), r.Header.Get("Idempotency-Key"), user, cmd)
	}
	if err != nil {
		hgFail(w, r, err)
		return
	}
	// 幂等重放可能查询已完成的任务，不能把既有状态误报成 pending。
	taskID, _ := hgID(id)
	task, err := handler.repo.HGDetail(r.Context(), taskID, user)
	if err != nil {
		hgFail(w, r, err)
		return
	}
	response.SuccessResult(w, r, map[string]any{"taskId": id, "status": task.Status, "estimatedRecipients": task.Total})
}

// HGDetail 只查询本人任务；不回显变量、正文或其他用户 ID。
func (handler *HGHandler) HGDetail(w http.ResponseWriter, r *http.Request) {
	user, err := hgActor(r)
	id, idErr := hgID(r.URL.Query().Get("id"))
	if err == nil {
		err = idErr
	}
	var task HGTask
	if err == nil {
		task, err = handler.repo.HGDetail(r.Context(), id, user)
	}
	if err != nil {
		hgFail(w, r, err)
		return
	}
	response.SuccessResult(w, r, task)
}

// HGMineList 仅按当前登录管理员与主键游标分页查询本人任务。
func (handler *HGHandler) HGMineList(w http.ResponseWriter, r *http.Request) {
	user, err := hgActor(r)
	cursor := uint64(math.MaxInt64)
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		value, parseErr := strconv.ParseUint(raw, 10, 64)
		if parseErr != nil || value == 0 || value > math.MaxInt64 {
			hgFail(w, r, HGErrInvalid)
			return
		}
		cursor = value
	}
	if err != nil {
		hgFail(w, r, err)
		return
	}
	items, next, err := handler.repo.HGMinePage(r.Context(), user, cursor)
	if err != nil {
		hgFail(w, r, err)
		return
	}
	response.SuccessResult(w, r, map[string]any{"list": items, "nextCursor": next})
}

// HGCancel 只阻止未执行用户；并发中的一次已成功落库投递不承诺撤回。
func (handler *HGHandler) HGCancel(w http.ResponseWriter, r *http.Request) {
	user, err := hgActor(r)
	id, idErr := hgID(r.URL.Query().Get("id"))
	if err == nil {
		err = idErr
	}
	if err == nil {
		err = handler.repo.HGCancel(r.Context(), id, user)
	}
	if err != nil {
		hgFail(w, r, err)
		return
	}
	response.SuccessResult(w, r, map[string]bool{"cancelled": true})
}
