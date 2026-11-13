package media

import (
	HGContext "MLC_GO/internal/pkg/hg_context"
	HGResponse "MLC_GO/internal/response"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// HGHandler 只暴露本人隔离上传和状态查询；不得返回隔离对象的下载地址。
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
