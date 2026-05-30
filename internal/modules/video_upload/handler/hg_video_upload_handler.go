package VideoUploadHandlerPackage

import (
	VideoUploadDtoPackage "MLC_GO/internal/modules/video_upload/dto"
	VideoUploadServicePackage "MLC_GO/internal/modules/video_upload/service"
	HGResponsePakcage "MLC_GO/internal/response"
	HGContextPackage "MLC_GO/internal/pkg/hg_context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"strconv"
	"strings"
)

const (
	maxMultipartHeaderBytes = 2 << 20
	maxUploadRequestBytes   = int64(4<<30) + maxMultipartHeaderBytes
)

// Handler 是 video_upload 模块的 HTTP 入口。
// 只负责鉴权上下文读取、HTTP 参数解析、错误码映射和统一响应，不直接写 SQL 或处理文件存储细节。
type Handler struct {
	service *VideoUploadServicePackage.Service
}

// NewHandler 创建视频投稿处理器，由 module assembly 统一注入 service。
func NewHandler(service *VideoUploadServicePackage.Service) *Handler {
	return &Handler{service: service}
}

// UploadVideo 接收 multipart/form-data 视频文件并创建/追加稿件分 P。
// 请求字段：file 必填，submissionId 可选，partNumber 可选。
// submissionId 为空时会创建新稿件；不为空时表示向已有稿件追加分 P。
func (h *Handler) UploadVideo(w http.ResponseWriter, r *http.Request) {
	userID, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePakcage.FailTokenInvalid(w, r, "unauthorized")
		return
	}
	if err := h.service.CheckUploadRateLimit(r.Context(), userID, clientIP(r)); err != nil {
		status, code := mapUploadError(err)
		w.WriteHeader(status)
		HGResponsePakcage.FailResult[string](w, r, code, err.Error())
		return
	}

	// 大视频上传不能使用 ParseMultipartForm，否则 net/http 会把超过内存阈值的内容写入系统临时文件，
	// 在高并发大文件场景会形成额外磁盘放大。这里直接流式读取 multipart part 并写入目标存储。
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadRequestBytes)
	file, fileName, fileSize, mimeType, fields, err := readUploadMultipart(r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InvalidParamCode, err.Error())
		return
	}
	if closer, ok := file.(io.Closer); ok {
		defer closer.Close()
	}
	partNumber, _ := strconv.ParseUint(fields["partNumber"], 10, 32)
	resp, err := h.service.UploadVideo(r.Context(), userID, file, fileName, fileSize, mimeType, strings.TrimSpace(fields["submissionId"]), uint32(partNumber))
	if err != nil {
		status, code := mapUploadError(err)
		w.WriteHeader(status)
		HGResponsePakcage.FailResult[string](w, r, code, err.Error())
		return
	}

	HGResponsePakcage.SuccessResult(w, r, resp)
}

// readUploadMultipart 以流式方式读取上传请求，避免把大文件缓存到内存或系统临时目录。
// 当前前端会先 append file，再 append submissionId/partNumber；为兼容字段顺序，这里同时支持从 URL query 兜底读取字段。
func readUploadMultipart(r *http.Request) (io.Reader, string, int64, string, map[string]string, error) {
	contentType := r.Header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
		return nil, "", 0, "", nil, errors.New("上传请求必须是 multipart/form-data")
	}

	reader := multipart.NewReader(r.Body, params["boundary"])
	fields := map[string]string{
		"submissionId": strings.TrimSpace(r.URL.Query().Get("submissionId")),
		"partNumber":   strings.TrimSpace(r.URL.Query().Get("partNumber")),
	}

	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, "", 0, "", nil, errors.New("读取上传表单失败")
		}

		if part.FileName() == "" {
			value, err := readSmallMultipartField(part)
			part.Close()
			if err != nil {
				return nil, "", 0, "", nil, err
			}
			fields[part.FormName()] = value
			continue
		}

		fileSize := r.ContentLength
		if fileSize < 0 {
			fileSize = 0
		}
		return part, part.FileName(), fileSize, part.Header.Get("Content-Type"), fields, nil
	}

	return nil, "", 0, "", nil, errors.New("缺少视频文件")
}

// readSmallMultipartField 只读取很小的文本字段，防止恶意 multipart 字段撑爆内存。
func readSmallMultipartField(part *multipart.Part) (string, error) {
	limited := io.LimitReader(part, maxMultipartHeaderBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return "", errors.New("读取上传字段失败")
	}
	if len(data) > maxMultipartHeaderBytes {
		return "", errors.New("上传字段过大")
	}
	return strings.TrimSpace(string(data)), nil
}

// SaveDraft 保存视频稿件草稿。
// 和 Submit 共用请求体，但这里强制 status=draft，避免前端伪造状态。
func (h *Handler) SaveDraft(w http.ResponseWriter, r *http.Request) {
	h.saveSubmissionWithStatus(w, r, "draft")
}

// Submit 提交视频稿件进入审核。
// 和 SaveDraft 共用请求体，但这里强制 status=reviewing。
func (h *Handler) Submit(w http.ResponseWriter, r *http.Request) {
	h.saveSubmissionWithStatus(w, r, "reviewing")
}

// saveSubmissionWithStatus 统一处理草稿保存和投稿审核，差异只在目标状态。
// 这样可以避免两个接口出现字段解析、校验和响应语义不一致。
func (h *Handler) saveSubmissionWithStatus(w http.ResponseWriter, r *http.Request, status string) {
	userID, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePakcage.FailTokenInvalid(w, r, "unauthorized")
		return
	}

	var req VideoUploadDtoPackage.SaveSubmissionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InvalidParamCode, "请求体格式错误")
		return
	}
	req.Status = status

	resp, err := h.service.SaveSubmission(r.Context(), userID, req)
	if err != nil {
		statusCode, code := mapSaveError(err)
		w.WriteHeader(statusCode)
		HGResponsePakcage.FailResult[string](w, r, code, err.Error())
		return
	}

	HGResponsePakcage.SuccessResult(w, r, resp)
}



// mapUploadError 把上传业务错误映射为 HTTP 状态码和统一业务码。
func mapUploadError(err error) (int, HGResponsePakcage.HGErrorCode) {
	switch {
	case errors.Is(err, VideoUploadServicePackage.ErrVideoFileEmpty),
		errors.Is(err, VideoUploadServicePackage.ErrVideoFileTooLarge),
		errors.Is(err, VideoUploadServicePackage.ErrVideoTypeInvalid):
		return http.StatusBadRequest, HGResponsePakcage.InvalidParamCode
	case errors.Is(err, VideoUploadServicePackage.ErrUploadRateLimited):
		return http.StatusTooManyRequests, HGResponsePakcage.InvalidParamCode
	default:
		return http.StatusInternalServerError, HGResponsePakcage.InternalErrorCode
	}
}

// mapSaveError 把稿件保存业务错误映射为 HTTP 状态码和统一业务码。
func mapSaveError(err error) (int, HGResponsePakcage.HGErrorCode) {
	switch {
	case errors.Is(err, VideoUploadServicePackage.ErrSubmissionInvalid),
		errors.Is(err, VideoUploadServicePackage.ErrVideoConfigInvalid):
		return http.StatusBadRequest, HGResponsePakcage.InvalidParamCode
	case errors.Is(err, VideoUploadServicePackage.ErrSubmitDuplicated):
		return http.StatusTooManyRequests, HGResponsePakcage.InvalidParamCode
	default:
		return http.StatusInternalServerError, HGResponsePakcage.InternalErrorCode
	}
}

/* 客户端 IP 地址 */
func clientIP(r *http.Request) string {
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		ips := strings.Split(xff, ",")
		ip := strings.TrimSpace(ips[0])
		if ip != "" {
			return ip
		}
	}

	if ip := strings.TrimSpace(r.Header.Get("X-Real-IP")); ip != "" {
		return ip
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}

	return r.RemoteAddr
}
