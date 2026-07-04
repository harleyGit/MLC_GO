package VideoUploadHandlerPackage

import (
	VideoUploadDtoPackage "MLC_GO/internal/modules/video_upload/dto"
	VideoUploadServicePackage "MLC_GO/internal/modules/video_upload/service"
	HGContextPackage "MLC_GO/internal/pkg/hg_context"
	HGResponsePakcage "MLC_GO/internal/response"
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
	/*
		为什么要额外加 2MB

		因为 multipart 解析时：
			boundary
			form-data header
			filename / content-type
			多字段结构

		这些都在 header 层
		👉 不加的话可能出现：
				body 没超，但 header 爆了
				parser OOM（解析器内存炸）
	*/
	maxMultipartHeaderBytes = 2 << 20                                // 2MB
	maxUploadRequestBytes   = int64(4<<30) + maxMultipartHeaderBytes // 4GB + 2MB，确保整个请求大小受控，防止恶意上传过大文件
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
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: code, Message: err.Error()})
		return
	}

	// 大视频上传不能使用 ParseMultipartForm，否则 net/http 会把超过内存阈值的内容写入系统临时文件，
	// 在高并发大文件场景会形成额外磁盘放大。这里直接流式读取 multipart part 并写入目标存储。
	// 限制 HTTP 请求 Body 的最大可读取大小，防止大包上传导致内存/磁盘/DoS 风险
	// 传入w可以在超限时 Go 会自动帮你写 HTTP 响应，默认返回：413 Request Entity Too Large
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadRequestBytes)
	file, fileName, fileSize, mimeType, fields, err := readUploadMultipart(r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParam.Code, Message: err.Error()})
		return
	}
	// file.(io.CLoser)看看file时候支持CLose()
	// 关闭文件流，防止资源泄漏；注意这里的 file 可能是 multipart.Part 或其他实现了 io.Reader 的类型，只有当它实现了 io.Closer 时才调用 Close。
	if closer, ok := file.(io.Closer); ok {
		// 函数结束时释放资源
		// multipart内部会维护：TCP连接、缓冲区、临时资源
		// 不关闭可能导致：fd泄漏、连接无法复用
		defer closer.Close()
	}
	// 解析分片号
	partNumber, _ := strconv.ParseUint(fields["partNumber"], 10, 32)
	// 上传视频文件并创建/追加稿件分 P，service 内部会处理文件保存、数据库记录和业务校验等逻辑。
	resp, err := h.service.UploadVideo(r.Context(), userID, file, fileName, fileSize, mimeType, strings.TrimSpace(fields["submissionId"]), uint32(partNumber))
	if err != nil {
		status, code := mapUploadError(err)
		w.WriteHeader(status)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: code, Message: err.Error()})
		return
	}

	HGResponsePakcage.SuccessResult(w, r, resp)
}

// readUploadMultipart 以流式方式读取上传请求，避免把大文件缓存到内存或系统临时目录。
// 当前前端会先 append file，再 append submissionId/partNumber；为兼容字段顺序，这里同时支持从 URL query 兜底读取字段。
// 返回值包括：视频文件流、文件名、文件大小、MIME 类型（文件类型）、其他字段（表单字段，如 submissionId/partNumber）和错误信息。
// TODO：这里只适合单个文件上传，当前端支持多文件上传时需要改成一次性读取所有字段，避免字段顺序导致的兼容问题。
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
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParam.Code, Message: "请求体格式错误"})
		return
	}
	req.Status = status

	resp, err := h.service.SaveSubmission(r.Context(), userID, req)
	if err != nil {
		statusCode, code := mapSaveError(err)
		w.WriteHeader(statusCode)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: code, Message: err.Error()})
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
		return http.StatusBadRequest, HGResponsePakcage.InvalidParam.Code
	case errors.Is(err, VideoUploadServicePackage.ErrUploadRateLimited):
		return http.StatusTooManyRequests, HGResponsePakcage.InvalidParam.Code
	default:
		return http.StatusInternalServerError, HGResponsePakcage.InternalError.Code
	}
}

// mapSaveError 把稿件保存业务错误映射为 HTTP 状态码和统一业务码。
func mapSaveError(err error) (int, HGResponsePakcage.HGErrorCode) {
	switch {
	case errors.Is(err, VideoUploadServicePackage.ErrSubmissionInvalid),
		errors.Is(err, VideoUploadServicePackage.ErrVideoConfigInvalid):
		return http.StatusBadRequest, HGResponsePakcage.InvalidParam.Code
	case errors.Is(err, VideoUploadServicePackage.ErrSubmitDuplicated):
		return http.StatusTooManyRequests, HGResponsePakcage.InvalidParam.Code
	default:
		return http.StatusInternalServerError, HGResponsePakcage.InternalError.Code
	}
}

// GetVideoList 获取已提交审核的视频列表。
// 支持游标分页，首次调用不传 cursor，后续使用响应中的 nextCursor 翻页。
func (h *Handler) GetVideoList(w http.ResponseWriter, r *http.Request) {
	_, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePakcage.FailTokenInvalid(w, r, "unauthorized")
		return
	}

	cursor := r.URL.Query().Get("cursor")
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))

	resp, err := h.service.GetVideoList(r.Context(), cursor, pageSize)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InternalError.Code, Message: err.Error()})
		return
	}

	HGResponsePakcage.SuccessResult(w, r, resp)
}

// UploadCover 接收 base64 编码的封面图，保存为文件并返回访问 URL。
// 请求体 JSON：{ "image": "data:image/jpeg;base64,..." }
// 响应 JSON：{ "code": 0, "data": { "url": "http://host/uploads/cover/xxx.jpg" } }
func (h *Handler) UploadCover(w http.ResponseWriter, r *http.Request) {
	userID, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePakcage.FailTokenInvalid(w, r, "unauthorized")
		return
	}

	var req struct {
		Image string `json:"image"`
	}
	// 解析 http 请求 body json 到 req 结构体
	// 创建一个 JSON 解码器，数据源是 HTTP 请求体流 r.Body。
	// r.Body：请求体数据流（前端 POST/PUT 提交的 body 内容）
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Image == "" {
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParam.Code, Message: "缺少 image 字段"})
		return
	}

	url, err := h.service.SaveCoverImage(r.Context(), userID, req.Image)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParam.Code, Message: err.Error()})
		return
	}

	HGResponsePakcage.SuccessResult(w, r, map[string]string{"url": url})
}

/* 客户端 IP 地址 */
func clientIP(r *http.Request) string {
	// 从 HTTP Header 读取 X-Forwarded-For: clientIP, proxy1, proxy2,这是标准代理链路 IP 记录头
	// 为什么有多个 IP？ 因为请求路径可能是：Client → Nginx → API Gateway → Go服务,所以变成 X-Forwarded-For: 真实客户端IP, NginxIP, GatewayIP
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// 获取的ip，比如：X-Forwarded-For: 1.2.3.4, 10.0.0.1, 10.0.0.2
		ips := strings.Split(xff, ",")
		// 1.2.3.4
		ip := strings.TrimSpace(ips[0])
		if ip != "" {
			return ip
		}
	}

	// 如果没有 XFF，再看 X-Real-IP， X-Real-IP通常单个 IP
	if ip := strings.TrimSpace(r.Header.Get("X-Real-IP")); ip != "" {
		return ip
	}

	// 这是 Go HTTP 的原始连接地址，比如： 192.168.1.10:54321 或 [::1]:54321
	// 使用 net.SplitHostPort 安全地分离主机和端口，支持 IPv6 地址
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}

	// 如果解析失败，返回原始地址（可能是没有端口的格式）
	return r.RemoteAddr
}
