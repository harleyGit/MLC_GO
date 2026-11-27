package VideoRecommendHandlerPackage

import (
	VideoRecommendServicePackage "MLC_GO/internal/modules/video_recommend/service"
	hgcontext "MLC_GO/internal/pkg/hg_context"
	HGResponsePakcage "MLC_GO/internal/response"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

// Handler 是认证视频推荐流的 HTTP 入口。
type Handler struct {
	service *VideoRecommendServicePackage.Service
}

// NewHandler 创建视频推荐处理器。
func NewHandler(service *VideoRecommendServicePackage.Service) *Handler {
	return &Handler{service: service}
}

// Feed 返回当前 JWT 用户的推荐视频游标页。
func (h *Handler) Feed(w http.ResponseWriter, r *http.Request) {
	userID, ok := hgcontext.CurrentUserID(r)
	if !ok {
		HGResponsePakcage.HGFailStatus(w, r, http.StatusUnauthorized, HGResponsePakcage.Unauthorized)
		return
	}
	pageSize := 0
	if value := strings.TrimSpace(r.URL.Query().Get("pageSize")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			HGResponsePakcage.HGFailStatus(w, r, http.StatusBadRequest, HGResponsePakcage.InvalidParam)
			return
		}
		pageSize = parsed
	}
	response, err := h.service.Feed(r.Context(), userID, r.URL.Query().Get("cursor"), pageSize)
	if err != nil {
		if errors.Is(err, VideoRecommendServicePackage.ErrInvalidRequest) {
			HGResponsePakcage.HGFailStatus(w, r, http.StatusBadRequest, HGResponsePakcage.InvalidParam)
			return
		}
		HGResponsePakcage.HGFailStatus(w, r, http.StatusServiceUnavailable, HGResponsePakcage.ServiceUnavailable)
		return
	}
	HGResponsePakcage.SuccessResult(w, r, response)
}
