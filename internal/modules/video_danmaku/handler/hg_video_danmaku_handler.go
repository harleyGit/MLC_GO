package VideoDanmakuHandlerPackage

import (
	VideoDanmakuDtoPackage "MLC_GO/internal/modules/video_danmaku/dto"
	VideoDanmakuRepositoryPackage "MLC_GO/internal/modules/video_danmaku/repository"
	VideoDanmakuServicePackage "MLC_GO/internal/modules/video_danmaku/service"
	HGContextPackage "MLC_GO/internal/pkg/hg_context"
	HGResponsePakcage "MLC_GO/internal/response"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

const hgMaxDanmakuBodyBytes = 4 << 10

// Handler 是认证视频弹幕 HTTP API 入口。
type Handler struct {
	service *VideoDanmakuServicePackage.Service
}

// NewHandler 创建弹幕 Handler。
func NewHandler(service *VideoDanmakuServicePackage.Service) *Handler {
	return &Handler{service: service}
}

// Create 持久化当前播放位置的弹幕。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		hgUnauthorized(w, r)
		return
	}
	var req VideoDanmakuDtoPackage.CreateRequest
	if !hgDecode(w, r, &req) {
		return
	}
	result, err := h.service.Create(r.Context(), userID, req)
	if err != nil {
		hgError(w, r, err)
		return
	}
	HGResponsePakcage.SuccessResult(w, r, result)
}

// List 返回指定视频有界时间窗内的弹幕。
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	fromMS, ok := hgUint32(w, r, "fromMs")
	if !ok {
		return
	}
	toMS, ok := hgUint32(w, r, "toMs")
	if !ok {
		return
	}
	pageSize := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("pageSize")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			hgError(w, r, VideoDanmakuServicePackage.ErrInvalidPageSize)
			return
		}
		pageSize = value
	}
	result, err := h.service.List(r.Context(), VideoDanmakuDtoPackage.ListRequest{VideoID: r.URL.Query().Get("videoId"), FromMS: fromMS, ToMS: toMS, Cursor: r.URL.Query().Get("cursor"), PageSize: pageSize})
	if err != nil {
		hgError(w, r, err)
		return
	}
	HGResponsePakcage.SuccessResult(w, r, result)
}

// Ticket 返回短期单次 WebSocket 连接票据。
func (h *Handler) Ticket(w http.ResponseWriter, r *http.Request) {
	userID, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		hgUnauthorized(w, r)
		return
	}
	var req VideoDanmakuDtoPackage.TicketRequest
	if !hgDecode(w, r, &req) {
		return
	}
	result, err := h.service.IssueTicket(r.Context(), userID, req.VideoID)
	if err != nil {
		hgError(w, r, err)
		return
	}
	HGResponsePakcage.SuccessResult(w, r, result)
}

func hgDecode(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, hgMaxDanmakuBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if decoder.Decode(target) != nil {
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.RequestBodyInvalid)
		return false
	}
	return true
}
func hgUint32(w http.ResponseWriter, r *http.Request, key string) (uint32, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return 0, true
	}
	value, err := strconv.ParseUint(raw, 10, 32)
	if err != nil {
		hgError(w, r, VideoDanmakuServicePackage.ErrInvalidWindow)
		return 0, false
	}
	return uint32(value), true
}
func hgUnauthorized(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusUnauthorized)
	HGResponsePakcage.FailTokenInvalid(w, r, "unauthorized")
}
func hgError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, VideoDanmakuRepositoryPackage.ErrVideoNotDanmakuEnabled) {
		w.WriteHeader(http.StatusForbidden)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParam.Code, Message: "当前视频不可发送弹幕"})
		return
	}
	if errors.Is(err, VideoDanmakuServicePackage.ErrInvalidTarget) || errors.Is(err, VideoDanmakuServicePackage.ErrInvalidContent) || errors.Is(err, VideoDanmakuServicePackage.ErrInvalidRequestID) || errors.Is(err, VideoDanmakuServicePackage.ErrInvalidProgress) || errors.Is(err, VideoDanmakuServicePackage.ErrInvalidStyle) || errors.Is(err, VideoDanmakuServicePackage.ErrInvalidWindow) || errors.Is(err, VideoDanmakuServicePackage.ErrInvalidPageSize) || errors.Is(err, VideoDanmakuServicePackage.ErrInvalidCursor) {
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParam.Code, Message: err.Error()})
		return
	}
	w.WriteHeader(http.StatusServiceUnavailable)
	HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.DatabaseError.Code, Message: "弹幕服务暂不可用"})
}
