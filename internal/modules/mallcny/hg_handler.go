package mallcny

import (
	cfg "MLC_GO/internal/pkg/config"
	ctxuser "MLC_GO/internal/pkg/hg_context"
	resp "MLC_GO/internal/response"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"time"
	"unicode"
	"unicode/utf8"
)

var hgIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// HGHandler 是独立人民币交易 API；仅模拟支付受 debug 开关控制，公开报价不等于支付成功。
type HGHandler struct {
	repo *HGRepository /* 与原商城同一个 MySQL 池。 */
}

func HGNewHandler(repo *HGRepository) *HGHandler { return &HGHandler{repo: repo} }

func hgID(s string) bool { return hgIDPattern.MatchString(s) }
func hgText(s string, max int) bool {
	if !utf8.ValidString(s) || utf8.RuneCountInString(s) < 1 || utf8.RuneCountInString(s) > max {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
func hgDecode(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	hgDecoder := json.NewDecoder(r.Body)
	hgDecoder.DisallowUnknownFields()
	if err := hgDecoder.Decode(target); err != nil {
		hgError(w, r, ErrHGInvalid)
		return false
	}
	var hgTrailing any
	if hgDecoder.Decode(&hgTrailing) != io.EOF {
		hgError(w, r, ErrHGInvalid)
		return false
	}
	return true
}
func hgError(w http.ResponseWriter, r *http.Request, err error) {
	hgStatus, hgMessage := http.StatusInternalServerError, "人民币商城暂不可用"
	switch {
	case errors.Is(err, ErrHGInvalid):
		hgStatus, hgMessage = http.StatusBadRequest, "请求参数无效"
	case errors.Is(err, ErrHGNotFound):
		hgStatus, hgMessage = http.StatusNotFound, "商品或订单不存在"
	case errors.Is(err, ErrHGQuoteChanged):
		hgStatus, hgMessage = http.StatusConflict, "价格已变化，请重新确认订单"
	case errors.Is(err, ErrHGConflict):
		hgStatus, hgMessage = http.StatusConflict, "库存、优惠券或拼团状态已变化"
	case errors.Is(err, ErrHGUnavailable):
		hgStatus, hgMessage = http.StatusForbidden, "模拟支付仅限隔离测试环境"
	}
	hgCode := resp.InvalidParam.Code
	if hgStatus >= 500 {
		hgCode = resp.InternalError.Code
	}
	w.WriteHeader(hgStatus)
	resp.FailResult[string](w, r, resp.HGErrorResult{Code: hgCode, Message: hgMessage})
}
func hgUser(w http.ResponseWriter, r *http.Request) (string, bool) {
	hgUser, hgOK := ctxuser.CurrentUserID(r)
	if !hgOK {
		w.WriteHeader(http.StatusUnauthorized)
		resp.FailTokenInvalid(w, r, "unauthorized")
	}
	return hgUser, hgOK
}
func hgContext(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), 5*time.Second)
}

// hgSandbox 防止未迁移的 pre/prod 环境访问仅在 debug 库创建的 CNY 表。
func hgSandbox(w http.ResponseWriter, r *http.Request) bool {
	if cfg.IsMallCNYSandboxEnabled() {
		return true
	}
	hgError(w, r, ErrHGUnavailable)
	return false
}

// HGList 按内部游标有界列出已审批商家的人民币商品；不混入 M币目录。
func (h *HGHandler) HGList(w http.ResponseWriter, r *http.Request) {
	if !hgSandbox(w, r) {
		return
	}
	hgCursor := uint64(math.MaxUint64)
	if hgText := r.URL.Query().Get("cursor"); hgText != "" {
		var err error
		hgCursor, err = strconv.ParseUint(hgText, 10, 64)
		if err != nil || hgCursor == 0 {
			hgError(w, r, ErrHGInvalid)
			return
		}
	}
	hgCtx, hgCancel := hgContext(r)
	defer hgCancel()
	hgItems, hgNext, err := h.repo.HGList(hgCtx, hgCursor)
	if err != nil {
		hgError(w, r, err)
		return
	}
	hgPage := struct {
		List       []HGProduct `json:"list"`
		NextCursor string      `json:"nextCursor"`
		HasMore    bool        `json:"hasMore"`
	}{List: hgItems, HasMore: hgNext > 0}
	if hgNext > 0 {
		hgPage.NextCursor = strconv.FormatUint(hgNext, 10)
	}
	resp.SuccessResult(w, r, hgPage)
}
func (h *HGHandler) HGDetail(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("productId")
	if !hgID(id) {
		hgError(w, r, ErrHGInvalid)
		return
	}
	if !hgSandbox(w, r) {
		return
	}
	hgCtx, hgCancel := hgContext(r)
	defer hgCancel()
	hgItem, err := h.repo.HGDetail(hgCtx, id)
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, hgItem)
}
func (h *HGHandler) HGPublish(w http.ResponseWriter, r *http.Request) {
	if !cfg.IsMallCNYSandboxEnabled() {
		hgError(w, r, ErrHGUnavailable)
		return
	}
	user, ok := hgUser(w, r)
	if !ok {
		return
	}
	var req HGCreateProduct
	if !hgDecode(w, r, &req) {
		return
	}
	// CNY 商品暂不提供图片绑定接口，不能让卖家冒用其他店铺已审核的 URL。
	if !hgText(req.Title, 128) || !hgText(req.SKUTitle, 128) || utf8.RuneCountInString(req.Description) > 4096 || !utf8.ValidString(req.Description) || req.PriceFen < 1 || req.PriceFen > 10000000 || req.Stock < 1 || req.Stock > 100000 || ((req.GroupPriceFen == nil) != (req.GroupMembers == 0)) || req.GroupMembers != 0 && (req.GroupMembers < 2 || req.GroupMembers > 10 || *req.GroupPriceFen < 1 || *req.GroupPriceFen > req.PriceFen) || req.ImageURL != "" {
		hgError(w, r, ErrHGInvalid)
		return
	}
	hgCtx, hgCancel := hgContext(r)
	defer hgCancel()
	hgItem, err := h.repo.HGPublish(hgCtx, user, req)
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, hgItem)
}
func (h *HGHandler) HGCreateCoupon(w http.ResponseWriter, r *http.Request) {
	if !cfg.IsMallCNYSandboxEnabled() {
		hgError(w, r, ErrHGUnavailable)
		return
	}
	user, ok := hgUser(w, r)
	if !ok {
		return
	}
	var req HGCreateCoupon
	if !hgDecode(w, r, &req) {
		return
	}
	if !hgID(req.ProductID) || req.ThresholdFen < 1 || req.ThresholdFen > 10000000 || req.DiscountFen < 1 || req.DiscountFen > req.ThresholdFen || req.Quantity < 1 || req.Quantity > 100000 || !req.ExpiresAt.After(time.Now().UTC()) || req.ExpiresAt.After(time.Now().UTC().Add(365*24*time.Hour)) {
		hgError(w, r, ErrHGInvalid)
		return
	}
	hgCtx, hgCancel := hgContext(r)
	defer hgCancel()
	hgCoupon, err := h.repo.HGCreateCoupon(hgCtx, user, req)
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, hgCoupon)
}
func (h *HGHandler) HGCoupons(w http.ResponseWriter, r *http.Request) {
	if !hgSandbox(w, r) {
		return
	}
	id := r.URL.Query().Get("productId")
	if !hgID(id) {
		hgError(w, r, ErrHGInvalid)
		return
	}
	hgCtx, hgCancel := hgContext(r)
	defer hgCancel()
	hgList, err := h.repo.HGCoupons(hgCtx, id)
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, hgList)
}
func (h *HGHandler) HGClaimCoupon(w http.ResponseWriter, r *http.Request) {
	if !cfg.IsMallCNYSandboxEnabled() {
		hgError(w, r, ErrHGUnavailable)
		return
	}
	user, ok := hgUser(w, r)
	if !ok {
		return
	}
	var req struct {
		ProductID string `json:"productId"`
		CouponID  string `json:"couponId"`
	}
	if !hgDecode(w, r, &req) {
		return
	}
	if !hgID(req.ProductID) || !hgID(req.CouponID) {
		hgError(w, r, ErrHGInvalid)
		return
	}
	hgCtx, hgCancel := hgContext(r)
	defer hgCancel()
	if err := h.repo.HGClaimCoupon(hgCtx, user, req.ProductID, req.CouponID); err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, struct {
		Claimed bool `json:"claimed"`
	}{true})
}
func hgValidOrder(req HGOrderRequest, buy bool) bool {
	if !hgID(req.ProductID) || !hgID(req.SKUID) || req.Quantity < 1 || req.Quantity > 10 || !(req.PurchaseType == "single" || req.PurchaseType == "group_create" || req.PurchaseType == "group_join") || req.PurchaseType == "group_join" && !hgID(req.GroupID) || req.PurchaseType != "group_join" && req.GroupID != "" || req.CouponID != "" && (!hgID(req.CouponID) || req.PurchaseType != "single") {
		return false
	}
	if buy && (!hgID(req.RequestID) || !hgText(req.Address, 512) || req.ExpectedPayableFen < 1) {
		return false
	}
	return true
}
func (h *HGHandler) HGPreview(w http.ResponseWriter, r *http.Request) {
	if !hgSandbox(w, r) {
		return
	}
	user, ok := hgUser(w, r)
	if !ok {
		return
	}
	var req HGOrderRequest
	if !hgDecode(w, r, &req) {
		return
	}
	if !hgValidOrder(req, false) {
		hgError(w, r, ErrHGInvalid)
		return
	}
	hgCtx, hgCancel := hgContext(r)
	defer hgCancel()
	hgQuote, err := h.repo.HGPreview(hgCtx, user, req)
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, hgQuote)
}
func (h *HGHandler) HGBuy(w http.ResponseWriter, r *http.Request) {
	if !cfg.IsMallCNYSandboxEnabled() {
		hgError(w, r, ErrHGUnavailable)
		return
	}
	user, ok := hgUser(w, r)
	if !ok {
		return
	}
	var req HGOrderRequest
	if !hgDecode(w, r, &req) {
		return
	}
	if !hgValidOrder(req, true) {
		hgError(w, r, ErrHGInvalid)
		return
	}
	hgCtx, hgCancel := hgContext(r)
	defer hgCancel()
	hgOrder, err := h.repo.HGBuy(hgCtx, user, req)
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, hgOrder)
}
func (h *HGHandler) HGGroups(w http.ResponseWriter, r *http.Request) {
	if !hgSandbox(w, r) {
		return
	}
	id := r.URL.Query().Get("productId")
	if !hgID(id) {
		hgError(w, r, ErrHGInvalid)
		return
	}
	hgCtx, hgCancel := hgContext(r)
	defer hgCancel()
	hgItems, err := h.repo.HGGroups(hgCtx, id)
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, hgItems)
}
func (h *HGHandler) HGGroup(w http.ResponseWriter, r *http.Request) {
	if !hgSandbox(w, r) {
		return
	}
	id := r.URL.Query().Get("groupId")
	if !hgID(id) {
		hgError(w, r, ErrHGInvalid)
		return
	}
	hgCtx, hgCancel := hgContext(r)
	defer hgCancel()
	hgGroup, err := h.repo.HGGroup(hgCtx, id)
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, hgGroup)
}
func (h *HGHandler) HGOrder(w http.ResponseWriter, r *http.Request) {
	if !hgSandbox(w, r) {
		return
	}
	user, ok := hgUser(w, r)
	if !ok {
		return
	}
	id := r.URL.Query().Get("orderId")
	if !hgID(id) {
		hgError(w, r, ErrHGInvalid)
		return
	}
	hgCtx, hgCancel := hgContext(r)
	defer hgCancel()
	hgOrder, err := h.repo.HGOrder(hgCtx, user, id)
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, hgOrder)
}
func (h *HGHandler) HGOrders(w http.ResponseWriter, r *http.Request) {
	if !hgSandbox(w, r) {
		return
	}
	user, ok := hgUser(w, r)
	if !ok {
		return
	}
	hgCursor := uint64(math.MaxUint64)
	if value := r.URL.Query().Get("cursor"); value != "" {
		var err error
		hgCursor, err = strconv.ParseUint(value, 10, 64)
		if err != nil || hgCursor == 0 {
			hgError(w, r, ErrHGInvalid)
			return
		}
	}
	hgCtx, hgCancel := hgContext(r)
	defer hgCancel()
	hgItems, hgNext, err := h.repo.HGOrders(hgCtx, user, hgCursor)
	if err != nil {
		hgError(w, r, err)
		return
	}
	hgPage := struct {
		List       []HGOrder `json:"list"`
		NextCursor string    `json:"nextCursor"`
		HasMore    bool      `json:"hasMore"`
	}{List: hgItems, HasMore: hgNext > 0}
	if hgNext > 0 {
		hgPage.NextCursor = strconv.FormatUint(hgNext, 10)
	}
	resp.SuccessResult(w, r, hgPage)
}
func (h *HGHandler) hgOrderAction(w http.ResponseWriter, r *http.Request, action string) {
	user, ok := hgUser(w, r)
	if !ok {
		return
	}
	var req struct {
		OrderID string `json:"orderId"`
	}
	if !hgDecode(w, r, &req) {
		return
	}
	if !hgID(req.OrderID) {
		hgError(w, r, ErrHGInvalid)
		return
	}
	if !cfg.IsMallCNYSandboxEnabled() {
		hgError(w, r, ErrHGUnavailable)
		return
	}
	hgCtx, hgCancel := hgContext(r)
	defer hgCancel()
	var hgOrder HGOrder
	var err error
	switch action {
	case "pay":
		hgOrder, err = h.repo.HGDebugPay(hgCtx, user, req.OrderID)
	case "refund":
		hgOrder, err = h.repo.HGDebugRefund(hgCtx, user, req.OrderID)
	case "cancel":
		hgOrder, err = h.repo.HGCancel(hgCtx, user, req.OrderID)
	}
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, hgOrder)
}
func (h *HGHandler) HGDebugPay(w http.ResponseWriter, r *http.Request) { h.hgOrderAction(w, r, "pay") }
func (h *HGHandler) HGDebugRefund(w http.ResponseWriter, r *http.Request) {
	h.hgOrderAction(w, r, "refund")
}
func (h *HGHandler) HGCancel(w http.ResponseWriter, r *http.Request) { h.hgOrderAction(w, r, "cancel") }

// HGCapabilities 告知前端模拟渠道是否在当前进程启用；不通过按钮隐藏替代服务端强制检查。
func (h *HGHandler) HGCapabilities(w http.ResponseWriter, r *http.Request) {
	resp.SuccessResult(w, r, struct {
		Sandbox bool `json:"sandbox"`
	}{cfg.IsMallCNYSandboxEnabled()})
}
