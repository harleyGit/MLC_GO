package VideoCommentHandlerPackage

import (
	VideoCommentDtoPackage "MLC_GO/internal/modules/video_comment/dto"
	VideoCommentRepositoryPackage "MLC_GO/internal/modules/video_comment/repository"
	VideoCommentServicePackage "MLC_GO/internal/modules/video_comment/service"
	HGContextPackage "MLC_GO/internal/pkg/hg_context"
	HGResponsePakcage "MLC_GO/internal/response"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

const hgMaxCommentBodyBytes = 8 << 10

// Handler 是认证视频评论 API 的 HTTP 入口。
type Handler struct {
	service *VideoCommentServicePackage.Service
}

// NewHandler 创建认证视频评论 API Handler。
func NewHandler(service *VideoCommentServicePackage.Service) *Handler {
	return &Handler{service: service}
}

// Create 校验认证身份和受限 JSON 请求体后创建顶级评论。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		hgUnauthorized(w, r)
		return
	}
	var req VideoCommentDtoPackage.CreateRequest
	if !hgDecodeJSON(w, r, &req) {
		return
	}
	response, err := h.service.Create(r.Context(), userID, req)
	if err != nil {
		hgWriteError(w, r, err)
		return
	}
	HGResponsePakcage.SuccessResult(w, r, response)
}

// List 解析 latest/hot 游标分页参数并返回当前用户可见的删除权限。
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		hgUnauthorized(w, r)
		return
	}
	pageSize := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("pageSize")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			hgWriteError(w, r, VideoCommentServicePackage.ErrInvalidPageSize)
			return
		}
		pageSize = parsed
	}
	response, err := h.service.List(r.Context(), userID, VideoCommentDtoPackage.ListRequest{
		SubmissionID: r.URL.Query().Get("submissionId"), Sort: strings.TrimSpace(r.URL.Query().Get("sort")),
		Cursor: strings.TrimSpace(r.URL.Query().Get("cursor")), PageSize: pageSize,
	})
	if err != nil {
		hgWriteError(w, r, err)
		return
	}
	HGResponsePakcage.SuccessResult(w, r, response)
}

// Delete 仅允许认证用户软删除自己的评论。
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		hgUnauthorized(w, r)
		return
	}
	var req VideoCommentDtoPackage.DeleteRequest
	if !hgDecodeJSON(w, r, &req) {
		return
	}
	response, err := h.service.Delete(r.Context(), userID, req)
	if err != nil {
		hgWriteError(w, r, err)
		return
	}
	HGResponsePakcage.SuccessResult(w, r, response)
}

func hgDecodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, hgMaxCommentBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.RequestBodyInvalid)
		return false
	}
	return true
}

func hgUnauthorized(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusUnauthorized)
	HGResponsePakcage.FailTokenInvalid(w, r, "unauthorized")
}

func hgWriteError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, VideoCommentServicePackage.ErrCommentNotFound) {
		w.WriteHeader(http.StatusNotFound)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParam.Code, Message: VideoCommentServicePackage.ErrCommentNotFound.Error()})
		return
	}
	if errors.Is(err, VideoCommentRepositoryPackage.ErrSubmissionNotCommentable) {
		w.WriteHeader(http.StatusForbidden)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParam.Code, Message: "当前视频不可评论"})
		return
	}
	if errors.Is(err, VideoCommentServicePackage.ErrInvalidTarget) || errors.Is(err, VideoCommentServicePackage.ErrInvalidContent) ||
		errors.Is(err, VideoCommentServicePackage.ErrInvalidRequestID) || errors.Is(err, VideoCommentServicePackage.ErrInvalidSort) ||
		errors.Is(err, VideoCommentServicePackage.ErrInvalidCursor) || errors.Is(err, VideoCommentServicePackage.ErrInvalidPageSize) {
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParam.Code, Message: err.Error()})
		return
	}
	w.WriteHeader(http.StatusServiceUnavailable)
	HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.DatabaseError.Code, Message: "评论服务暂不可用"})
}
