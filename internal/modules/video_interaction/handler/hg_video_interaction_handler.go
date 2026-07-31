package VideoInteractionHandlerPackage

import (
	CoinRepositoryPackage "MLC_GO/internal/modules/coin/repository"
	VideoInteractionDtoPackage "MLC_GO/internal/modules/video_interaction/dto"
	VideoInteractionRepositoryPackage "MLC_GO/internal/modules/video_interaction/repository"
	VideoInteractionServicePackage "MLC_GO/internal/modules/video_interaction/service"
	HGContextPackage "MLC_GO/internal/pkg/hg_context"
	HGResponsePakcage "MLC_GO/internal/response"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

const hgMaxInteractionBodyBytes = 8 << 10

// Handler 是视频详情互动 API 入口。
type Handler struct {
	service *VideoInteractionServicePackage.Service
}

func NewHandler(service *VideoInteractionServicePackage.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) State(w http.ResponseWriter, r *http.Request) {
	userID, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		hgUnauthorized(w, r)
		return
	}
	state, err := h.service.GetState(r.Context(), userID, strings.TrimSpace(r.URL.Query().Get("submissionId")), strings.TrimSpace(r.URL.Query().Get("authorId")))
	if err != nil {
		hgWriteError(w, r, err)
		return
	}
	HGResponsePakcage.SuccessResult(w, r, state)
}

func (h *Handler) Like(w http.ResponseWriter, r *http.Request)     { h.action(w, r, "like") }
func (h *Handler) Coin(w http.ResponseWriter, r *http.Request)     { h.action(w, r, "coin") }
func (h *Handler) Favorite(w http.ResponseWriter, r *http.Request) { h.action(w, r, "favorite") }
func (h *Handler) Share(w http.ResponseWriter, r *http.Request)    { h.action(w, r, "share") }

func (h *Handler) action(w http.ResponseWriter, r *http.Request, action string) {
	userID, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		hgUnauthorized(w, r)
		return
	}
	var req VideoInteractionDtoPackage.ActionRequest
	if !hgDecodeJSON(w, r, &req) {
		return
	}
	req.Action = action
	response, err := h.service.SetVideoInteraction(r.Context(), userID, req)
	if err != nil {
		hgWriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
	HGResponsePakcage.SuccessResult(w, r, response)
}

func (h *Handler) Follow(w http.ResponseWriter, r *http.Request) {
	userID, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		hgUnauthorized(w, r)
		return
	}
	var req VideoInteractionDtoPackage.FollowRequest
	if !hgDecodeJSON(w, r, &req) {
		return
	}
	response, err := h.service.SetFollow(r.Context(), userID, req)
	if err != nil {
		hgWriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
	HGResponsePakcage.SuccessResult(w, r, response)
}

func hgDecodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, hgMaxInteractionBodyBytes)
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
	status := http.StatusBadRequest
	code := HGResponsePakcage.InvalidParam.Code
	if errors.Is(err, CoinRepositoryPackage.ErrHGInsufficientBalance) || errors.Is(err, VideoInteractionRepositoryPackage.ErrInsufficientCoinBalance) {
		w.WriteHeader(http.StatusForbidden)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InsufficientBalance)
		return
	}
	if errors.Is(err, CoinRepositoryPackage.ErrHGBusinessLimit) || errors.Is(err, VideoInteractionRepositoryPackage.ErrCoinLimitExceeded) {
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InvalidParam)
		return
	}
	if errors.Is(err, CoinRepositoryPackage.ErrHGIdempotencyConflict) || errors.Is(err, VideoInteractionRepositoryPackage.ErrCoinIdempotencyConflict) {
		w.WriteHeader(http.StatusConflict)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.InvalidParam)
		return
	}
	if !errors.Is(err, VideoInteractionServicePackage.ErrInvalidAction) &&
		!errors.Is(err, VideoInteractionServicePackage.ErrInvalidTarget) &&
		!errors.Is(err, VideoInteractionServicePackage.ErrInvalidQuantity) &&
		!errors.Is(err, VideoInteractionServicePackage.ErrInvalidRequestID) &&
		!errors.Is(err, VideoInteractionServicePackage.ErrCannotFollowSelf) {
		status = http.StatusServiceUnavailable
		code = HGResponsePakcage.MQError.Code
	}
	w.WriteHeader(status)
	HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: code, Message: err.Error()})
}
