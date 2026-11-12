package media

import (
	"errors"
	"strings"
)

var (
	HGErrInvalid  = errors.New("媒体上传参数不合法")
	HGErrConflict = errors.New("媒体上传状态冲突")
	HGErrNotFound = errors.New("媒体资源不存在或无权访问")
)

// HGUploadRequest 是媒体上传初始化协议；purpose 是必填业务场景，不接受客户端指定存储位置。
type HGUploadRequest struct {
	Purpose        string `json:"purpose"`         // 必填，服务端白名单内的具体业务用途。
	MIME           string `json:"mime_type"`       // 必填，声明的 MIME；审核时须重新探测真实内容。
	Size           int64  `json:"file_size"`       // 必填，上传字节数，必须大于零且不超过用途上限。
	SHA256         string `json:"sha256"`          // 必填，64 位小写十六进制；审核时须重新计算。
	IdempotencyKey string `json:"idempotency_key"` // 必填，调用者范围内稳定的重试键，最长 64 字符。
}

type hgPolicy struct {
	biz   string            // 所属业务域，用于对象路径隔离；不是客户端可写字段。
	kind  string            // 内容类别，用于隔离路径与 MIME 交叉校验。
	max   int64             // 单次申请最大字节数；不等于用户总配额。
	mimes map[string]string // MIME 到固定扩展名的映射，不使用原始文件名。
}

var hgPolicies = map[string]hgPolicy{
	"user_avatar":      {"user", "avatar", 10 << 20, map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/webp": "webp"}},
	"user_cover":       {"user", "cover", 10 << 20, map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/webp": "webp"}},
	"user_identity":    {"user", "identity", 20 << 20, map[string]string{"image/jpeg": "jpg", "image/png": "png", "application/pdf": "pdf"}},
	"product_main":     {"shop", "product", 10 << 20, map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/webp": "webp"}},
	"product_detail":   {"shop", "detail", 10 << 20, map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/webp": "webp"}},
	"product_sku":      {"shop", "sku", 10 << 20, map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/webp": "webp"}},
	"shop_logo":        {"shop", "logo", 10 << 20, map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/webp": "webp"}},
	"chat_image":       {"chat", "image", 10 << 20, map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/webp": "webp"}},
	"chat_audio":       {"chat", "audio", 128 << 20, map[string]string{"audio/mpeg": "mp3", "audio/ogg": "ogg", "audio/mp4": "m4a"}},
	"chat_video":       {"chat", "video", 300 << 20, map[string]string{"video/mp4": "mp4", "video/quicktime": "mov"}},
	"chat_file":        {"chat", "file", 128 << 20, map[string]string{"application/pdf": "pdf", "application/zip": "zip"}},
	"video_source":     {"video", "source", 300 << 20, map[string]string{"video/mp4": "mp4", "video/quicktime": "mov"}},
	"video_cover":      {"video", "cover", 10 << 20, map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/webp": "webp"}},
	"video_subtitle":   {"video", "subtitle", 2 << 20, map[string]string{"text/vtt": "vtt", "application/x-subrip": "srt"}},
	"comment_image":    {"comment", "image", 10 << 20, map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/webp": "webp", "image/gif": "gif"}},
	"comment_video":    {"comment", "video", 100 << 20, map[string]string{"video/mp4": "mp4"}},
	"support_evidence": {"support", "evidence", 128 << 20, map[string]string{"image/jpeg": "jpg", "image/png": "png", "video/mp4": "mp4", "application/pdf": "pdf"}},
	"marketing_poster": {"marketing", "poster", 10 << 20, map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/webp": "webp"}},
}

func hgValidateRequest(req HGUploadRequest) (hgPolicy, string, error) {
	policy, ok := hgPolicies[req.Purpose]
	ext, okMIME := policy.mimes[req.MIME]
	if !ok || !okMIME || req.Size < 1 || req.Size > policy.max || len(req.SHA256) != 64 || len(req.IdempotencyKey) < 1 || len(req.IdempotencyKey) > 64 {
		return hgPolicy{}, "", HGErrInvalid
	}
	for _, c := range req.SHA256 {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return hgPolicy{}, "", HGErrInvalid
		}
	}
	for _, c := range req.IdempotencyKey {
		if !strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-", c) {
			return hgPolicy{}, "", HGErrInvalid
		}
	}
	return policy, ext, nil
}
