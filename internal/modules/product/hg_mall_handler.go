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
	case errors.Is(err, ErrHGConflict), errors.Is(err, ErrHGState), errors.Is(err, ErrHGStock), errors.Is(err, ErrHGFunds):
		status, message = http.StatusConflict, "当前状态已变化，请刷新重试"
		if errors.Is(err, ErrHGFunds) {
			message = "M币余额不足或有效批次不足"
		}
		if errors.Is(err, ErrHGStock) {
			message = "库存不足、金额超出单笔上限或商品不可购买"
		}
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

// HGBuy 从当前登录用户读取身份；下单预留库存十五分钟，不立即扣币。
func (h *HGHandler) HGBuy(w http.ResponseWriter, r *http.Request) {
	user, ok := hgAuth(w, r)
	if !ok {
		return
	}
	var req HGBuyRequest
	if !hgDecode(w, r, &req) {
		return
	}
	order, err := h.service.HGBuy(r.Context(), user, req)
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, order)
}

// HGPendingOrders 只返回登录买家的待支付及过期取消记录，支持有界游标分页。
func (h *HGHandler) HGPendingOrders(w http.ResponseWriter, r *http.Request) {
	user, ok := hgAuth(w, r)
	if !ok {
		return
	}
	page, err := h.service.HGPendingOrders(r.Context(), user, r.URL.Query().Get("cursor"))
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, page)
}

// HGPay 仅按本人订单号支付既有待支付订单，金额由服务端快照读取。
func (h *HGHandler) HGPay(w http.ResponseWriter, r *http.Request) {
	user, ok := hgAuth(w, r)
	if !ok {
		return
	}
	var req struct {
		OrderID string `json:"orderId"` // 本人未过期的待支付订单业务标识，不接受客户端金额。
	}
	if !hgDecode(w, r, &req) {
		return
	}
	order, err := h.service.HGPay(r.Context(), user, req.OrderID)
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, order)
}

func (h *HGHandler) HGOrders(w http.ResponseWriter, r *http.Request)         { h.hgOrders(w, r, false) }
func (h *HGHandler) HGMerchantOrders(w http.ResponseWriter, r *http.Request) { h.hgOrders(w, r, true) }
func (h *HGHandler) hgOrders(w http.ResponseWriter, r *http.Request, merchant bool) {
	user, ok := hgAuth(w, r)
	if !ok {
		return
	}
	page, err := h.service.HGOrders(r.Context(), user, r.URL.Query().Get("cursor"), merchant)
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, page)
}

func (h *HGHandler) HGOrder(w http.ResponseWriter, r *http.Request)         { h.hgOrder(w, r, false) }
func (h *HGHandler) HGMerchantOrder(w http.ResponseWriter, r *http.Request) { h.hgOrder(w, r, true) }
func (h *HGHandler) hgOrder(w http.ResponseWriter, r *http.Request, merchant bool) {
	user, ok := hgAuth(w, r)
	if !ok {
		return
	}
	order, err := h.service.HGOrder(r.Context(), user, r.URL.Query().Get("orderId"), merchant)
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, order)
}

func (h *HGHandler) HGShip(w http.ResponseWriter, r *http.Request) {
	user, ok := hgAuth(w, r)
	if !ok {
		return
	}
	var req HGShipRequest
	if !hgDecode(w, r, &req) {
		return
	}
	if err := h.service.HGShip(r.Context(), user, req); err != nil {
		hgError(w, r, err)
		return
	}
	order, err := h.service.HGOrder(r.Context(), user, req.OrderID, true)
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, order)
}

func (h *HGHandler) HGReceive(w http.ResponseWriter, r *http.Request) {
	user, ok := hgAuth(w, r)
	if !ok {
		return
	}
	var req struct {
		OrderID string `json:"orderId"`
	}
	if !hgDecode(w, r, &req) {
		return
	}
	if err := h.service.HGReceive(r.Context(), user, req.OrderID); err != nil {
		hgError(w, r, err)
		return
	}
	order, err := h.service.HGOrder(r.Context(), user, req.OrderID, false)
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, order)
}

func (h *HGHandler) HGRefund(w http.ResponseWriter, r *http.Request) {
	user, ok := hgAuth(w, r)
	if !ok {
		return
	}
	var req HGRefundRequest
	if !hgDecode(w, r, &req) {
		return
	}
	if err := h.service.HGRefund(r.Context(), user, req); err != nil {
		hgError(w, r, err)
		return
	}
	order, err := h.service.HGOrder(r.Context(), user, req.OrderID, false)
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, order)
}
