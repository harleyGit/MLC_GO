package media

import (
	HGContext "MLC_GO/internal/pkg/hg_context"
	HGResponse "MLC_GO/internal/response"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"
)

// HGHandler 除本人隔离上传与状态外，仅提供受 IAM 限制的新旧媒体元数据盘点，不向管理员提供对象读取地址。
type HGHandler struct {
	service *HGService // 媒体元数据服务；依赖受信用户上下文。
}

// HGNewHandler 构造媒体 HTTP 入口，不从请求接受 owner、bucket 或 object key。
func HGNewHandler(service *HGService) *HGHandler { return &HGHandler{service: service} }

func (h *HGHandler) hgUser(w http.ResponseWriter, r *http.Request) (string, bool) {
	owner, ok := HGContext.CurrentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		HGResponse.FailTokenInvalid(w, r, "请先登录")
	}
	return owner, ok
}

func hgFail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, HGErrInvalid):
		w.WriteHeader(http.StatusBadRequest)
		HGResponse.FailResult[string](w, r, HGResponse.InvalidParam)
	case errors.Is(err, HGErrConflict):
		w.WriteHeader(http.StatusConflict)
		HGResponse.FailResult[string](w, r, HGResponse.InvalidParam)
	case errors.Is(err, HGErrNotFound):
		w.WriteHeader(http.StatusNotFound)
		HGResponse.FailResult[string](w, r, HGResponse.InvalidParam)
	default:
		w.WriteHeader(http.StatusServiceUnavailable)
		HGResponse.FailResult[string](w, r, HGResponse.DatabaseError)
	}
}

// HGInitUpload 要求显式上传业务类型；JSON 限制为 4 KiB，不接收任意额外字段。
func (h *HGHandler) HGInitUpload(w http.ResponseWriter, r *http.Request) {
	owner, ok := h.hgUser(w, r)
	if !ok {
		return
	}
	var req HGUploadRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&req) != nil || decoder.Decode(new(any)) != io.EOF {
		hgFail(w, r, HGErrInvalid)
		return
	}
	result, err := h.service.HGInit(r.Context(), owner, req)
	if err != nil {
		hgFail(w, r, err)
		return
	}
	HGResponse.SuccessResult(w, r, result)
}

// HGCompleteUpload 仅在本人会话真实对象长度匹配后转为 processing，不等于审核通过。
func (h *HGHandler) HGCompleteUpload(w http.ResponseWriter, r *http.Request) {
	owner, ok := h.hgUser(w, r)
	if !ok {
		return
	}
	var req struct {
		UploadID string `json:"upload_id"` // 本人上传会话 UUID，必须与数据库会话归属一致。
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&req) != nil || decoder.Decode(new(any)) != io.EOF {
		hgFail(w, r, HGErrInvalid)
		return
	}
	result, err := h.service.HGComplete(r.Context(), owner, req.UploadID)
	if err != nil {
		hgFail(w, r, err)
		return
	}
	HGResponse.SuccessResult(w, r, result)
}

// HGAssetStatus 返回本人对象元数据；未经独立审核时没有公开下载能力。
func (h *HGHandler) HGAssetStatus(w http.ResponseWriter, r *http.Request) {
	owner, ok := h.hgUser(w, r)
	if !ok {
		return
	}
	result, err := h.service.HGAssetStatus(r.Context(), owner, r.URL.Query().Get("asset_id"))
	if err != nil {
		hgFail(w, r, err)
		return
	}
	HGResponse.SuccessResult(w, r, result)
}

// HGOwnerDownload 只向经 JWT 验证的原上传者签发短时私有链接；不承担 IM 接收方或商家全员权限。
func (h *HGHandler) HGOwnerDownload(w http.ResponseWriter, r *http.Request) {
	owner, ok := h.hgUser(w, r)
	if !ok {
		return
	}
	link, err := h.service.HGOwnerDownload(r.Context(), owner, r.URL.Query().Get("asset_id"))
	if err != nil {
		hgFail(w, r, err)
		return
	}
	HGResponse.SuccessResult(w, r, map[string]any{"download_url": link, "expires_in": 30})
}

// HGInventoryList 全平台新旧媒体只读盘点；RouteSpec 先验证活动管理员和 media:inventory:read。
// kind=空/image/video/audio/file，limit 默认 20、最大 50；游标绑定种类并限制长度。
func (h *HGHandler) HGInventoryList(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.hgUser(w, r); !ok {
		return
	}
	query := r.URL.Query()
	if len(query) > 3 || len(query["kind"]) > 1 || len(query["cursor"]) > 1 || len(query["limit"]) > 1 {
		hgFail(w, r, HGErrInvalid)
		return
	}
	for key := range query {
		if key != "kind" && key != "cursor" && key != "limit" {
			hgFail(w, r, HGErrInvalid)
			return
		}
	}
	limit := 20
	if raw := query.Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 50 {
			hgFail(w, r, HGErrInvalid)
			return
		}
		limit = parsed
	}
	if !hgInventoryKindValid(query.Get("kind")) || len(query.Get("cursor")) > 90 {
		hgFail(w, r, HGErrInvalid)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()
	page, err := h.service.HGInventory(ctx, query.Get("kind"), query.Get("cursor"), limit)
	if err != nil {
		hgFail(w, r, err)
		return
	}
	HGResponse.SuccessResult(w, r, page)
}
