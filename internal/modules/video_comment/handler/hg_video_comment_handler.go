package VideoCommentHandlerPackage

import (
	VideoCommentDtoPackage "MLC_GO/internal/modules/video_comment/dto"
	VideoCommentRepositoryPackage "MLC_GO/internal/modules/video_comment/repository"
	VideoCommentServicePackage "MLC_GO/internal/modules/video_comment/service"
	HGContextPackage "MLC_GO/internal/pkg/hg_context"
	HGResponsePakcage "MLC_GO/internal/response"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
)

const hgMaxCommentBodyBytes = 8 << 10
const hgMaxCommentImageBytes = 5 << 20

// Handler 是认证视频评论 API 的 HTTP 入口。
type Handler struct {
	service *VideoCommentServicePackage.Service
}

// Replies 返回指定根评论的时间正序回复页。
func (h *Handler) Replies(w http.ResponseWriter, r *http.Request) {
	userID, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		hgUnauthorized(w, r)
		return
	}
	pageSize, ok := hgParsePageSize(w, r)
	if !ok {
		return
	}
	response, err := h.service.ListReplies(r.Context(), userID, VideoCommentDtoPackage.RepliesRequest{
		RootCommentID: r.URL.Query().Get("rootCommentId"), Cursor: strings.TrimSpace(r.URL.Query().Get("cursor")), PageSize: pageSize,
	})
	if err != nil {
		hgWriteError(w, r, err)
		return
	}
	HGResponsePakcage.SuccessResult(w, r, response)
}

// Reaction 将当前用户对评论的关系设置为请求中的最终状态。
func (h *Handler) Reaction(w http.ResponseWriter, r *http.Request) {
	userID, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		hgUnauthorized(w, r)
		return
	}
	var req VideoCommentDtoPackage.ReactionRequest
	if !hgDecodeJSON(w, r, &req) {
		return
	}
	response, err := h.service.SetReaction(r.Context(), userID, req)
	if err != nil {
		hgWriteError(w, r, err)
		return
	}
	HGResponsePakcage.SuccessResult(w, r, response)
}

// Image 接收非 multipart 的 raw image body；API Guard 按空 body 签名约定校验。
func (h *Handler) Image(w http.ResponseWriter, r *http.Request) {
	userID, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		hgUnauthorized(w, r)
		return
	}
	if r.ContentLength <= 0 || r.ContentLength > hgMaxCommentImageBytes {
		hgWriteError(w, r, VideoCommentServicePackage.ErrInvalidImageUpload)
		return
	}
	ext := strings.TrimSpace(r.URL.Query().Get("ext"))
	if ext == "" {
		ext = hgImageExtFromContentType(r.Header.Get("Content-Type"))
	}
	r.Body = http.MaxBytesReader(w, r.Body, hgMaxCommentImageBytes)
	var reader io.Reader = r.Body
	if ext == "" {
		var err error
		ext, reader, err = VideoCommentServicePackage.DetectCommentImageExt(r.Body)
		if err != nil {
			hgWriteError(w, r, err)
			return
		}
	}
	response, err := h.service.UploadImage(r.Context(), userID, hgRemoteIP(r), reader, r.ContentLength, ext)
	if err != nil {
		hgWriteError(w, r, err)
		return
	}
	HGResponsePakcage.SuccessResult(w, r, response)
}

// NewHandler 创建认证视频评论 API Handler。
func NewHandler(service *VideoCommentServicePackage.Service) *Handler {
	return &Handler{service: service}
}

// Create 校验认证身份和受限 JSON 请求体后创建顶级评论或回复。
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
	pageSize, ok := hgParsePageSize(w, r)
	if !ok {
		return
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
	if errors.Is(err, VideoCommentServicePackage.ErrImageRateLimited) {
		w.WriteHeader(http.StatusTooManyRequests)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParam.Code, Message: err.Error()})
		return
	}
	if errors.Is(err, VideoCommentServicePackage.ErrImageQuotaExceeded) || errors.Is(err, VideoCommentRepositoryPackage.ErrImageQuotaExceeded) {
		w.WriteHeader(http.StatusConflict)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParam.Code, Message: VideoCommentServicePackage.ErrImageQuotaExceeded.Error()})
		return
	}
	if errors.Is(err, VideoCommentServicePackage.ErrCommentHasReplies) {
		w.WriteHeader(http.StatusConflict)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParam.Code, Message: VideoCommentServicePackage.ErrCommentHasReplies.Error()})
		return
	}
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
	if errors.Is(err, VideoCommentRepositoryPackage.ErrParentNotAvailable) || errors.Is(err, VideoCommentServicePackage.ErrInvalidParent) || errors.Is(err, VideoCommentRepositoryPackage.ErrCommentNotAvailable) {
		w.WriteHeader(http.StatusNotFound)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParam.Code, Message: "评论不存在或不可用"})
		return
	}
	if errors.Is(err, VideoCommentServicePackage.ErrInvalidTarget) || errors.Is(err, VideoCommentServicePackage.ErrInvalidContent) ||
		errors.Is(err, VideoCommentServicePackage.ErrInvalidRequestID) || errors.Is(err, VideoCommentServicePackage.ErrInvalidSort) ||
		errors.Is(err, VideoCommentServicePackage.ErrInvalidCursor) || errors.Is(err, VideoCommentServicePackage.ErrInvalidPageSize) ||
		errors.Is(err, VideoCommentServicePackage.ErrInvalidImageURLs) || errors.Is(err, VideoCommentServicePackage.ErrInvalidReaction) ||
		errors.Is(err, VideoCommentServicePackage.ErrInvalidImageUpload) {
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParam.Code, Message: err.Error()})
		return
	}
	w.WriteHeader(http.StatusServiceUnavailable)
	HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.DatabaseError.Code, Message: "评论服务暂不可用"})
}

func hgRemoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}

func hgParsePageSize(w http.ResponseWriter, r *http.Request) (int, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get("pageSize"))
	if raw == "" {
		return 0, true
	}
	pageSize, err := strconv.Atoi(raw)
	if err != nil {
		hgWriteError(w, r, VideoCommentServicePackage.ErrInvalidPageSize)
		return 0, false
	}
	return pageSize, true
}

func hgImageExtFromContentType(contentType string) string {
	switch {
	case strings.HasPrefix(contentType, "image/jpeg"):
		return "jpg"
	case strings.HasPrefix(contentType, "image/png"):
		return "png"
	case strings.HasPrefix(contentType, "image/webp"):
		return "webp"
	default:
		return ""
	}
}
