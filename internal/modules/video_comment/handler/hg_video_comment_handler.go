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
	"net/http"
	"net/netip"
	"strconv"
	"strings"
)

const hgMaxCommentBodyBytes = 8 << 10
const hgMaxCommentImageBytes = 5 << 20

// Handler 是认证视频评论 API 的 HTTP 入口。
type Handler struct {
	service *VideoCommentServicePackage.Service
	// trustedProxyCIDRs 只描述可写入转发头的直连代理网段；为空时所有请求都只使用 RemoteAddr。
	trustedProxyCIDRs []netip.Prefix
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
	response, err := h.service.UploadImage(r.Context(), userID, hgClientIP(r, h.trustedProxyCIDRs), reader, r.ContentLength, ext)
	if err != nil {
		hgWriteError(w, r, err)
		return
	}
	HGResponsePakcage.SuccessResult(w, r, response)
}

// NewHandler 创建认证视频评论 API Handler，并复制启动期已校验的可信代理 CIDR，避免调用方后续修改切片。
func NewHandler(service *VideoCommentServicePackage.Service, trustedProxyCIDRs ...netip.Prefix) *Handler {
	return &Handler{service: service, trustedProxyCIDRs: append([]netip.Prefix(nil), trustedProxyCIDRs...)}
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

// hgClientIP 仅在 TCP 直连来源可信时解析代理头，并从右向左剥离可信代理，返回最右侧非可信地址。
// 非可信来源、畸形 X-Forwarded-For 或不支持的 IP:port/quoted 格式均 fail closed 到 RemoteAddr，防止伪造来源绕过 IP 限流。
func hgClientIP(r *http.Request, trustedProxyCIDRs []netip.Prefix) string {
	peer := hgParseRemoteAddr(r.RemoteAddr)
	if !peer.IsValid() {
		return strings.TrimSpace(r.RemoteAddr)
	}
	if !hgIPInPrefixes(peer, trustedProxyCIDRs) {
		return peer.String()
	}
	forwardedHeader := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if forwardedHeader == "" {
		if realIP, err := netip.ParseAddr(strings.TrimSpace(r.Header.Get("X-Real-IP"))); err == nil {
			return realIP.Unmap().String()
		}
		return peer.String()
	}
	forwarded := strings.Split(forwardedHeader, ",")
	var leftmost netip.Addr
	// 代理按追加语义写入 XFF；从最靠近应用的一跳向左验证，不能直接信任客户端可控的首项。
	for index := len(forwarded) - 1; index >= 0; index-- {
		candidate, err := netip.ParseAddr(strings.TrimSpace(forwarded[index]))
		if err != nil {
			return peer.String()
		}
		candidate = candidate.Unmap()
		leftmost = candidate
		if !hgIPInPrefixes(candidate, trustedProxyCIDRs) {
			return candidate.String()
		}
	}
	if leftmost.IsValid() {
		return leftmost.String()
	}
	return peer.String()
}

// hgParseRemoteAddr 接受 net/http 常见的 IP:port 和测试场景纯 IP，并统一 IPv4-mapped IPv6 表示。
func hgParseRemoteAddr(remoteAddr string) netip.Addr {
	value := strings.TrimSpace(remoteAddr)
	if addressPort, err := netip.ParseAddrPort(value); err == nil {
		return addressPort.Addr().Unmap()
	}
	if address, err := netip.ParseAddr(value); err == nil {
		return address.Unmap()
	}
	return netip.Addr{}
}

// hgIPInPrefixes 判断地址是否属于启动期规范化后的任一可信代理网段。
func hgIPInPrefixes(address netip.Addr, prefixes []netip.Prefix) bool {
	for _, prefix := range prefixes {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
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
