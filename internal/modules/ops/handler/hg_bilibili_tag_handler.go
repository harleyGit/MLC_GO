package OpsHandlerPackage

import (
	OpsDtoPackage "MLC_GO/internal/modules/ops/dto"
	HGContextPackage "MLC_GO/internal/pkg/hg_context"
	HGResponsePakcage "MLC_GO/internal/response"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// CreateBilibiliTag 创建 Bilibili 动画标签。
// Handler 仅负责读取登录用户、解码请求和响应映射，名称唯一性与缓存一致性交由 Service/Repository。
func (h *Handler) CreateBilibiliTag(w http.ResponseWriter, r *http.Request) {
	userID, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePakcage.FailTokenInvalid(w, r, "unauthorized")
		return
	}
	var req OpsDtoPackage.BilibiliTagRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBilibiliTagError(w, r, http.StatusBadRequest, "请求体格式错误")
		return
	}
	resp, err := h.service.CreateBilibiliTag(r.Context(), userID, req)
	if err != nil {
		writeBilibiliTagServiceError(w, r, err)
		return
	}
	HGResponsePakcage.SuccessResult(w, r, resp)
}

// UpdateBilibiliTag 更新 Bilibili 动画标签。
func (h *Handler) UpdateBilibiliTag(w http.ResponseWriter, r *http.Request) {
	userID, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePakcage.FailTokenInvalid(w, r, "unauthorized")
		return
	}
	var req OpsDtoPackage.UpdateBilibiliTagRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBilibiliTagError(w, r, http.StatusBadRequest, "请求体格式错误")
		return
	}
	resp, err := h.service.UpdateBilibiliTag(r.Context(), userID, req)
	if err != nil {
		writeBilibiliTagServiceError(w, r, err)
		return
	}
	HGResponsePakcage.SuccessResult(w, r, resp)
}

// DeleteBilibiliTag 删除 Bilibili 动画标签。
func (h *Handler) DeleteBilibiliTag(w http.ResponseWriter, r *http.Request) {
	userID, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePakcage.FailTokenInvalid(w, r, "unauthorized")
		return
	}
	var req OpsDtoPackage.DeleteBilibiliTagRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBilibiliTagError(w, r, http.StatusBadRequest, "请求体格式错误")
		return
	}
	if err := h.service.DeleteBilibiliTag(r.Context(), userID, req); err != nil {
		writeBilibiliTagServiceError(w, r, err)
		return
	}
	HGResponsePakcage.SuccessResult[interface{}](w, r, nil)
}

// GetBilibiliTagList 获取 Bilibili 动画标签列表。
// cursor/pageSize 服务于运维管理分页；activeOnly=true 服务于动画页的启用标签读取。
func (h *Handler) GetBilibiliTagList(w http.ResponseWriter, r *http.Request) {
	if _, ok := HGContextPackage.CurrentUserID(r); !ok {
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePakcage.FailTokenInvalid(w, r, "unauthorized")
		return
	}
	cursor, _ := strconv.ParseInt(r.URL.Query().Get("cursor"), 10, 64)
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	activeOnly := r.URL.Query().Get("activeOnly") == "true"
	resp, err := h.service.GetBilibiliTagList(r.Context(), cursor, pageSize, activeOnly)
	if err != nil {
		writeBilibiliTagServiceError(w, r, err)
		return
	}
	HGResponsePakcage.SuccessResult(w, r, resp)
}

// writeBilibiliTagServiceError 将稳定业务错误映射为 HTTP 状态，并屏蔽数据库内部错误上下文。
func writeBilibiliTagServiceError(w http.ResponseWriter, r *http.Request, err error) {
	message := err.Error()
	status := http.StatusBadRequest
	if strings.Contains(message, "已存在") {
		status = http.StatusConflict
	} else if strings.Contains(message, "不存在") {
		status = http.StatusNotFound
	} else if strings.Contains(message, "create bilibili") || strings.Contains(message, "query bilibili") {
		status = http.StatusInternalServerError
		message = "标签服务暂不可用"
	}
	writeBilibiliTagError(w, r, status, message)
}

// writeBilibiliTagError 使用项目统一响应结构输出标签接口错误。
func writeBilibiliTagError(w http.ResponseWriter, r *http.Request, status int, message string) {
	w.WriteHeader(status)
	code := HGResponsePakcage.InvalidParam.Code
	if status >= http.StatusInternalServerError {
		code = HGResponsePakcage.InternalError.Code
	}
	HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: code, Message: message})
}
