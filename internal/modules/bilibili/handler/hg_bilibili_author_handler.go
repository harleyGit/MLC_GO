package BilibiliHandlerPackage

import (
	BilibiliServicePackage "MLC_GO/internal/modules/bilibili/service"
	HGResponsePakcage "MLC_GO/internal/response"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

// Handler 是 Bilibili 作者空间公开读接口的 HTTP 入口。
type Handler struct {
	service *BilibiliServicePackage.Service
}

// NewHandler 创建作者空间处理器。
func NewHandler(service *BilibiliServicePackage.Service) *Handler { return &Handler{service: service} }

// GetProfile 返回作者公开资料。
func (h *Handler) GetProfile(w http.ResponseWriter, r *http.Request) {
	value, err := h.service.GetProfile(r.Context(), r.URL.Query().Get("userId"))
	if err != nil {
		hgWriteAuthorError(w, r, err)
		return
	}
	HGResponsePakcage.SuccessResult(w, r, value)
}

// GetStats 返回作者粉丝、关注和公开视频统计。
func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	value, err := h.service.GetStats(r.Context(), r.URL.Query().Get("userId"))
	if err != nil {
		hgWriteAuthorError(w, r, err)
		return
	}
	HGResponsePakcage.SuccessResult(w, r, value)
}

// GetVideos 返回作者公开视频游标页。
func (h *Handler) GetVideos(w http.ResponseWriter, r *http.Request) {
	value, err := h.service.GetVideos(r.Context(), r.URL.Query().Get("userId"), r.URL.Query().Get("cursor"), hgAuthorPageSize(r))
	if err != nil {
		hgWriteAuthorError(w, r, err)
		return
	}
	HGResponsePakcage.SuccessResult(w, r, value)
}

// GetHomepage 返回作者空间首屏聚合数据。
func (h *Handler) GetHomepage(w http.ResponseWriter, r *http.Request) {
	value, err := h.service.GetHomepage(r.Context(), r.URL.Query().Get("userId"), r.URL.Query().Get("cursor"), hgAuthorPageSize(r))
	if err != nil {
		hgWriteAuthorError(w, r, err)
		return
	}
	HGResponsePakcage.SuccessResult(w, r, value)
}

func hgAuthorPageSize(r *http.Request) int {
	value, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("pageSize")))
	if err != nil {
		return 0
	}
	return value
}

func hgWriteAuthorError(w http.ResponseWriter, r *http.Request, err error) {
	result := HGResponsePakcage.InternalError
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, BilibiliServicePackage.ErrInvalidAuthorRequest):
		result = HGResponsePakcage.InvalidParam
		status = http.StatusBadRequest
	case errors.Is(err, sql.ErrNoRows):
		result = HGResponsePakcage.UserNotFound
		status = http.StatusNotFound
	}
	w.WriteHeader(status)
	HGResponsePakcage.FailResult[string](w, r, result)
}
