package product

import (
	ctxuser "MLC_GO/internal/pkg/hg_context"
	resp "MLC_GO/internal/response"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// HGHandler 提供商城目录、商家申请、商品草稿和数据库 RBAC 审核接口。
type HGHandler struct{ service *HGService }

func HGNewHandler(service *HGService) *HGHandler { return &HGHandler{service: service} }

func hgAuth(w http.ResponseWriter, r *http.Request) (string, bool) {
	id, ok := ctxuser.CurrentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		resp.FailTokenInvalid(w, r, "unauthorized")
	}
	return id, ok
}

func hgDecode(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		hgError(w, r, ErrHGInvalid)
		return false
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		hgError(w, r, ErrHGInvalid)
		return false
	}
	return true
}

func hgError(w http.ResponseWriter, r *http.Request, err error) {
	status, message := http.StatusInternalServerError, "商城服务暂不可用"
	switch {
	case errors.Is(err, ErrHGInvalid):
		status, message = http.StatusBadRequest, "商城参数无效"
	case errors.Is(err, ErrHGForbidden):
		status, message = http.StatusForbidden, "无权操作"
	case errors.Is(err, ErrHGConflict):
		status, message = http.StatusConflict, "当前状态已变化，请刷新重试"
	case errors.Is(err, sql.ErrNoRows):
		status, message = http.StatusNotFound, "未找到记录"
	}
	code := resp.InvalidParam.Code
	if status >= 500 {
		code = resp.InternalError.Code
	}
	w.WriteHeader(status)
	resp.FailResult[string](w, r, resp.HGErrorResult{Code: code, Message: message})
}

func (h *HGHandler) HGList(w http.ResponseWriter, r *http.Request) {
	page, err := h.service.List(r.Context(), r.URL.Query().Get("category"), r.URL.Query().Get("cursor"))
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, page)
}

func (h *HGHandler) HGDetail(w http.ResponseWriter, r *http.Request) {
	item, err := h.service.Detail(r.Context(), r.URL.Query().Get("productId"))
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, item)
}

func (h *HGHandler) HGMerchant(w http.ResponseWriter, r *http.Request) {
	user, ok := hgAuth(w, r)
	if !ok {
		return
	}
	m, err := h.service.Merchant(r.Context(), user)
	if errors.Is(err, sql.ErrNoRows) {
		resp.SuccessResult[any](w, r, nil)
		return
	}
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, m)
}

func (h *HGHandler) HGApply(w http.ResponseWriter, r *http.Request) {
	user, ok := hgAuth(w, r)
	if !ok {
		return
	}
	var req struct {
		ShopName string `json:"shopName"`
	}
	if !hgDecode(w, r, &req) {
		return
	}
	m, err := h.service.Apply(r.Context(), user, req.ShopName)
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, m)
}

func (h *HGHandler) HGPublish(w http.ResponseWriter, r *http.Request) {
	user, ok := hgAuth(w, r)
	if !ok {
		return
	}
	var draft HGProductDraft
	if !hgDecode(w, r, &draft) {
		return
	}
	item, err := h.service.Publish(r.Context(), user, draft)
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, item)
}

func (h *HGHandler) HGMine(w http.ResponseWriter, r *http.Request) {
	user, ok := hgAuth(w, r)
	if !ok {
		return
	}
	page, err := h.service.Mine(r.Context(), user, r.URL.Query().Get("cursor"))
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, page)
}

func (h *HGHandler) HGPendingMerchants(w http.ResponseWriter, r *http.Request) {
	user, ok := hgAuth(w, r)
	if !ok {
		return
	}
	page, err := h.service.PendingMerchants(r.Context(), user, r.URL.Query().Get("cursor"))
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, page)
}

func (h *HGHandler) HGPendingProducts(w http.ResponseWriter, r *http.Request) {
	user, ok := hgAuth(w, r)
	if !ok {
		return
	}
	page, err := h.service.PendingProducts(r.Context(), user, r.URL.Query().Get("cursor"))
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, page)
}

func (h *HGHandler) HGReviewMerchant(w http.ResponseWriter, r *http.Request) {
	user, ok := hgAuth(w, r)
	if !ok {
		return
	}
	var req struct {
		MerchantID string `json:"merchantId"`
		Approve    *bool  `json:"approve"`
	}
	if !hgDecode(w, r, &req) {
		return
	}
	if req.Approve == nil {
		hgError(w, r, ErrHGInvalid)
		return
	}
	if err := h.service.ReviewMerchant(r.Context(), user, req.MerchantID, *req.Approve); err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, struct {
		MerchantID string `json:"merchantId"`
	}{req.MerchantID})
}

func (h *HGHandler) HGReviewProduct(w http.ResponseWriter, r *http.Request) {
	user, ok := hgAuth(w, r)
	if !ok {
		return
	}
	var req struct {
		ProductID string `json:"productId"`
		Approve   *bool  `json:"approve"`
	}
	if !hgDecode(w, r, &req) {
		return
	}
	if req.Approve == nil {
		hgError(w, r, ErrHGInvalid)
		return
	}
	if err := h.service.ReviewProduct(r.Context(), user, req.ProductID, *req.Approve); err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, struct {
		ProductID string `json:"productId"`
	}{req.ProductID})
}
