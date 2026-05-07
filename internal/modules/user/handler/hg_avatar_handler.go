package UserHandlerPackage

import (
	HGResponsePakcage "MLC_GO/internal/response"
	"net/http"
	"strings"
)

// AvatarUploadResponse 是头像上传响应。
// IsNew 用于告诉客户端本次是否生成了新的头像对象，便于前端决定是否刷新缓存。
type AvatarUploadResponse struct {
	AvatarURL string `json:"avatarUrl"`
	IsNew     bool   `json:"isNew"`
}

// Avatar 是头像资源的 HTTP 方法分发入口。
// 使用一个路径承载 GET/POST 可以减少路由数量，但真正业务仍拆到 UploadAvatar/GetAvatar，便于测试和维护。
func (h *HGUserHandler) Avatar(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.UploadAvatar(w, r)
	case http.MethodGet:
		h.GetAvatar(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.MethodNotAllowCode, "method not allowed")
	}
}

// UploadAvatar 上传用户头像。
// handler 只做 HTTP 级约束：用户 ID、ContentLength、MaxBytesReader 和格式推断；存储细节由 AvatarService 处理。
func (h *HGUserHandler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUpdateUserID(r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InvalidParamCode, err.Error())
		return
	}

	if r.ContentLength <= 0 {
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InvalidParamCode, "图片数据为空")
		return
	}
	if r.ContentLength > 10<<20 {
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InvalidParamCode, "图片大小超过限制")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)

	ext := r.URL.Query().Get("ext")
	if ext == "" {
		ext = getExtFromContentType(r.Header.Get("Content-Type"))
	}
	if ext == "" {
		ext = "png"
	}

	result, err := h.avatarSvc.UploadAvatarFromReader(r.Context(), userID, r.Body, r.ContentLength, ext)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InternalErrorCode, "上传头像失败: "+err.Error())
		return
	}

	HGResponsePakcage.SuccessResult(w, r, AvatarUploadResponse{
		AvatarURL: result.AvatarURL,
		IsNew:     result.IsNew,
	})
}

// GetAvatar 获取用户头像 URL。
// 读取逻辑交给 AvatarService，后续即使头像存储迁移到对象存储/CDN，handler 也不需要改。
func (h *HGUserHandler) GetAvatar(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUpdateUserID(r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InvalidParamCode, err.Error())
		return
	}

	avatarURL, err := h.avatarSvc.GetAvatarURL(r.Context(), userID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InternalErrorCode, "获取头像失败: "+err.Error())
		return
	}

	HGResponsePakcage.SuccessResult(w, r, map[string]string{"avatarUrl": avatarURL})
}

// getExtFromContentType 从 Content-Type 推断图片格式。
// 只做格式映射，不做安全校验；真正的文件内容校验应放在 upload/service 层统一处理。
func getExtFromContentType(contentType string) string {
	switch {
	case strings.Contains(contentType, "image/png"):
		return "png"
	case strings.Contains(contentType, "image/jpeg"):
		return "jpg"
	case strings.Contains(contentType, "image/gif"):
		return "gif"
	case strings.Contains(contentType, "image/webp"):
		return "webp"
	default:
		return ""
	}
}
