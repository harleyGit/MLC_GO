package commerce

import (
	config "MLC_GO/internal/pkg/config"
	resp "MLC_GO/internal/response"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
)

// hgDebugTrading 仅本机隔离 debug 的显式交易开关可读写订单；pre/prod 不访问尚未迁移的交易表。
func (h *HGHandler) hgDebugTrading(w http.ResponseWriter, r *http.Request) bool {
	if h.orders != nil && config.IsCommerceDebugTradingEnabled() {
		return true
	}
	hgError(w, r, ErrHGPaymentUnavailable)
	return false
}

// HGInventory 读取本店某 SKU 权威库存，版本由服务端提供供人工盘点比对。
func (h *HGHandler) HGInventory(w http.ResponseWriter, r *http.Request) {
	if !h.hgDebugTrading(w, r) {
		return
	}
	user, ok := hgAuth(w, r)
	if !ok {
		return
	}
	ctx, cancel := hgContext(r)
	defer cancel()
	stock, err := h.orders.HGStock(ctx, user, r.URL.Query().Get("productId"), r.URL.Query().Get("skuCode"))
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, stock)
}

// HGInventoryAdjust 按本人获批店铺、乐观锁和同事务审计进行 debug 人工盘点。
func (h *HGHandler) HGInventoryAdjust(w http.ResponseWriter, r *http.Request) {
	if !h.hgDebugTrading(w, r) {
		return
	}
	user, ok := hgAuth(w, r)
	if !ok {
		return
	}
	var req HGStockAdjustment
	if !hgDecode(w, r, &req) {
		return
	}
	ctx, cancel := hgContext(r)
	defer cancel()
	if err := h.orders.HGAdjustStock(ctx, user, req); err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, struct{}{})
}

// HGOrderPreview 在 debug 显式开关下只读服务端报价，不锁库存、不创建支付事实。
func (h *HGHandler) HGOrderPreview(w http.ResponseWriter, r *http.Request) {
	if !h.hgDebugTrading(w, r) {
		return
	}
	user, ok := hgAuth(w, r)
	if !ok {
		return
	}
	var req HGReserveRequest
	if !hgDecode(w, r, &req) {
		return
	}
	ctx, cancel := hgContext(r)
	defer cancel()
	quote, err := h.orders.HGPreview(ctx, user, req)
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, quote)
}

// HGOrderCreate 仅 debug 预占库存并创建待支付订单；没有渠道时不得声称订单已支付。
func (h *HGHandler) HGOrderCreate(w http.ResponseWriter, r *http.Request) {
	if !h.hgDebugTrading(w, r) {
		return
	}
	user, ok := hgAuth(w, r)
	if !ok {
		return
	}
	var req HGReserveRequest
	if !hgDecode(w, r, &req) {
		return
	}
	ctx, cancel := hgContext(r)
	defer cancel()
	order, err := h.orders.HGReserve(ctx, user, req)
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, order)
}

// HGOrderDetail 仅返回本人订单收件快照，其他用户不可探测是否存在。
func (h *HGHandler) HGOrderDetail(w http.ResponseWriter, r *http.Request) {
	if !h.hgDebugTrading(w, r) {
		return
	}
	user, ok := hgAuth(w, r)
	if !ok {
		return
	}
	ctx, cancel := hgContext(r)
	defer cancel()
	order, err := h.orders.HGOrder(ctx, user, r.URL.Query().Get("orderId"))
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, order)
}

// HGOrderList 仅本人稳定游标分页，列表不泄露收件地址。
func (h *HGHandler) HGOrderList(w http.ResponseWriter, r *http.Request) {
	if !h.hgDebugTrading(w, r) {
		return
	}
	user, ok := hgAuth(w, r)
	if !ok {
		return
	}
	ctx, cancel := hgContext(r)
	defer cancel()
	page, err := h.orders.HGOrders(ctx, user, r.URL.Query().Get("cursor"), false)
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, page)
}

// HGOrderCancel 仅买家本人关闭未支付订单；重复调用不重复返还预占。
func (h *HGHandler) HGOrderCancel(w http.ResponseWriter, r *http.Request) {
	if !h.hgDebugTrading(w, r) {
		return
	}
	user, ok := hgAuth(w, r)
	if !ok {
		return
	}
	var req struct {
		OrderID string `json:"orderId"` /* 生产 CNY 订单 ID；不得使用旧商城订单。 */
	}
	if !hgDecode(w, r, &req) {
		return
	}
	ctx, cancel := hgContext(r)
	defer cancel()
	if err := h.orders.HGCancel(ctx, user, req.OrderID); err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, struct{}{})
}

// HGOrderPayment 创建待回调模拟支付尝试；客户端提交成功不等于已经到账。
func (h *HGHandler) HGOrderPayment(w http.ResponseWriter, r *http.Request) {
	if !h.hgDebugTrading(w, r) {
		return
	}
	user, ok := hgAuth(w, r)
	if !ok {
		return
	}
	var req struct {
		OrderID string `json:"orderId"` /* 本人待支付的生产 CNY 订单。 */
	}
	if !hgDecode(w, r, &req) {
		return
	}
	ctx, cancel := hgContext(r)
	defer cancel()
	result, err := h.orders.HGStartSimulatedPayment(ctx, user, req.OrderID)
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, result)
}

// HGSimulatedCallback 只受 debug 开关+管理员权限保护，验证原始正文 HMAC 后才允许修改收款事实。
func (h *HGHandler) HGSimulatedCallback(w http.ResponseWriter, r *http.Request) {
	if !h.hgDebugTrading(w, r) {
		return
	}
	if _, ok := hgAuth(w, r); !ok {
		return
	}
	secret := []byte(os.Getenv("MLC_COMMERCE_SIMULATED_WEBHOOK_KEY"))
	if len(secret) < 32 {
		hgError(w, r, ErrHGPaymentUnavailable)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	raw, err := io.ReadAll(r.Body)
	if err != nil || len(raw) == 0 {
		hgError(w, r, ErrHGInvalid)
		return
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var event HGSimulatedEvent
	if decoder.Decode(&event) != nil || decoder.Decode(new(any)) != io.EOF {
		hgError(w, r, ErrHGInvalid)
		return
	}
	ctx, cancel := hgContext(r)
	defer cancel()
	result, err := h.orders.HGApplySimulatedEvent(ctx, secret, raw, r.Header.Get("X-Simulated-Signature"))
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, result)
}

// HGSimulatedSettle 是 debug 管理员显式模拟收款操作，后端生成并验签一次性事件，不向网页暴露模拟密钥。
func (h *HGHandler) HGSimulatedSettle(w http.ResponseWriter, r *http.Request) {
	if !h.hgDebugTrading(w, r) {
		return
	}
	if _, ok := hgAuth(w, r); !ok {
		return
	}
	secret := []byte(os.Getenv("MLC_COMMERCE_SIMULATED_WEBHOOK_KEY"))
	if len(secret) < 32 {
		hgError(w, r, ErrHGPaymentUnavailable)
		return
	}
	var req struct {
		OrderID string `json:"orderId"` /* 模拟订单 ID，仅管理员操作。 */
	}
	if !hgDecode(w, r, &req) {
		return
	}
	ctx, cancel := hgContext(r)
	defer cancel()
	result, err := h.orders.HGSettleSimulated(ctx, secret, req.OrderID)
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, result)
}

// HGOrderRefund 模拟已付未发货原路退款；不得混入真实支付流水。
func (h *HGHandler) HGOrderRefund(w http.ResponseWriter, r *http.Request) {
	if !h.hgDebugTrading(w, r) {
		return
	}
	user, ok := hgAuth(w, r)
	if !ok {
		return
	}
	var req struct {
		OrderID string `json:"orderId"` /* 本人已确认模拟支付且未发货的订单。 */
	}
	if !hgDecode(w, r, &req) {
		return
	}
	ctx, cancel := hgContext(r)
	defer cancel()
	if err := h.orders.HGRefundSimulated(ctx, user, req.OrderID); err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, struct{}{})
}

// HGMerchantOrders 商家只能读取已付本店的订单列表，收件信息仅详情返回。
func (h *HGHandler) HGMerchantOrders(w http.ResponseWriter, r *http.Request) {
	if !h.hgDebugTrading(w, r) {
		return
	}
	user, ok := hgAuth(w, r)
	if !ok {
		return
	}
	ctx, cancel := hgContext(r)
	defer cancel()
	page, err := h.orders.HGOrders(ctx, user, r.URL.Query().Get("cursor"), true)
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, page)
}

// HGMerchantOrderDetail 限本店支付后详情，用于发货前确认收件快照。
func (h *HGHandler) HGMerchantOrderDetail(w http.ResponseWriter, r *http.Request) {
	if !h.hgDebugTrading(w, r) {
		return
	}
	user, ok := hgAuth(w, r)
	if !ok {
		return
	}
	ctx, cancel := hgContext(r)
	defer cancel()
	order, err := h.orders.HGSellerOrder(ctx, user, r.URL.Query().Get("orderId"))
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, order)
}

// HGSimulatedReconcile 仅管理员按订单查看隔离支付事实漂移，绝不在线改写权威状态。
func (h *HGHandler) HGSimulatedReconcile(w http.ResponseWriter, r *http.Request) {
	if !h.hgDebugTrading(w, r) {
		return
	}
	if _, ok := hgAuth(w, r); !ok {
		return
	}
	ctx, cancel := hgContext(r)
	defer cancel()
	result, err := h.orders.HGReconcileSimulated(ctx, r.URL.Query().Get("orderId"))
	if err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, result)
}

// HGOrderShipment 仅在 debug 已验签模拟支付且没有退款时允许本店发货；pre/prod 均拒绝。
func (h *HGHandler) HGOrderShipment(w http.ResponseWriter, r *http.Request) {
	if !h.hgDebugTrading(w, r) {
		return
	}
	user, ok := hgAuth(w, r)
	if !ok {
		return
	}
	var req HGShipRequest
	if !hgDecode(w, r, &req) {
		return
	}
	ctx, cancel := hgContext(r)
	defer cancel()
	if err := h.orders.HGShipSimulated(ctx, user, req); err != nil {
		hgError(w, r, err)
		return
	}
	resp.SuccessResult(w, r, struct{}{})
}
