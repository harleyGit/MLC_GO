package commerce

import (
	config "MLC_GO/internal/pkg/config"
	q "MLC_GO/internal/pkg/mysql/queries"
	util "MLC_GO/internal/pkg/utils"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"
)

// HGSimulatedEvent 是独立 debug 模拟渠道 V1 回调正文，绝不冒充 ALIPAY/WECHAT；签名基于原始 JSON 字节。
type HGSimulatedEvent struct {
	EventID           string `json:"eventId"`           // 本次模拟收款事件唯一 ID，重放须同正文。
	PaymentID         string `json:"paymentId"`         // 由本人下单后生成的 SIMULATED 支付尝试 ID。
	OrderID           string `json:"orderId"`           // 生产 CNY 交易订单业务 ID。
	ExternalPaymentID string `json:"externalPaymentId"` // 唯一模拟渠道流水 ID，不可复用或冒充真实流水。
	Currency          string `json:"currency"`          // 固定 CNY。
	AmountCent        int64  `json:"amountCent"`        // 已模拟收款人民币分，必须与服务端快照一致。
	Status            string `json:"status"`            // 固定 SUCCESS；退款独立由服务端模拟原路执行。
}

// HGSimulatedPayment 是用户启动的 debug 支付尝试；只有已验签回调才确认到账。
type HGSimulatedPayment struct {
	PaymentID  string `json:"paymentId"`  // 模拟支付尝试业务 ID。
	OrderID    string `json:"orderId"`    // 对应生产 CNY 待支付订单。
	AmountCent int64  `json:"amountCent"` // 人民币整数分，不代表已支付。
	Status     uint8  `json:"status"`     // 1待回调、2已收到模拟付款。
}

// HGSimulatedReceipt 是已验签事件的结果；REFUNDED_* 表示模拟收款后模拟退款，不是实际外部资金往来。
type HGSimulatedReceipt struct {
	Result string `json:"result"` // PAID、REFUNDED_LATE 或 REFUNDED_EXPIRED。
}

// HGReconciliation 是单笔模拟订单的只读核对结果；异常只报告不在线篡改权威账本。
type HGReconciliation struct {
	OrderID           string `json:"orderId"`           // 请求指定的订单 ID。
	OrderStatus       uint8  `json:"orderStatus"`       // CNY 订单状态。
	TradeStatus       uint8  `json:"tradeStatus"`       // 统一订单状态。
	PaymentStatus     uint8  `json:"paymentStatus"`     // SIMULATED 支付状态，无支付尝试时为0。
	ReservationStatus uint8  `json:"reservationStatus"` // 1预占、2释放、3售出。
	EventResult       string `json:"eventResult"`       // 无事件时为空。
	RefundStatus      uint8  `json:"refundStatus"`      // 模拟退款单状态，无退款时为0。
	Drift             bool   `json:"drift"`             // 只报告不自动修复状态漂移。
}

// HGSimulatedSignature 使用隔离 debug 专用高熵密钥 HMAC-SHA256 对原始回调正文签名，测试可复核；生产不得复用。
func HGSimulatedSignature(secret, raw []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(raw)
	return hex.EncodeToString(mac.Sum(nil))
}

func hgVerifySimulated(secret, raw []byte, signature string) bool {
	if len(secret) < 32 || len(signature) != 64 {
		return false
	}
	provided, err := hex.DecodeString(signature)
	if err != nil {
		return false
	}
	expected, _ := hex.DecodeString(HGSimulatedSignature(secret, raw))
	return subtle.ConstantTimeCompare(expected, provided) == 1
}

// HGStartSimulatedPayment 仅已登录买家对自己的待支付订单发起一次模拟尝试，同请求重试不二次建单。
func (s *HGOrderStore) HGStartSimulatedPayment(ctx context.Context, user, orderID string) (HGSimulatedPayment, error) {
	if !config.IsCommerceDebugTradingEnabled() {
		return HGSimulatedPayment{}, ErrHGPaymentUnavailable
	}
	if user == "" || !hgID(orderID) {
		return HGSimulatedPayment{}, ErrHGInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return HGSimulatedPayment{}, err
	}
	defer tx.Rollback()
	var owner, product, sku, merchant string
	var qty uint32
	var status uint8
	var expiry time.Time
	var total int64
	if err = tx.QueryRowContext(ctx, q.HGCommerceSimOrderLockSQL, orderID).Scan(&owner, &product, &sku, &qty, &status, &expiry, &total, &merchant); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return HGSimulatedPayment{}, ErrHGForbidden
		}
		return HGSimulatedPayment{}, err
	}
	if owner != user {
		return HGSimulatedPayment{}, ErrHGForbidden
	}
	var result HGSimulatedPayment
	result.OrderID = orderID
	result.AmountCent = total
	err = tx.QueryRowContext(ctx, q.HGCommerceSimPaymentGetSQL, orderID).Scan(&result.PaymentID, &owner, &result.AmountCent, &result.Status)
	if err == nil {
		if owner != user || result.AmountCent != total {
			return HGSimulatedPayment{}, ErrHGConflict
		}
		return result, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return HGSimulatedPayment{}, err
	}
	if status != 1 || !expiry.After(time.Now().UTC()) {
		return HGSimulatedPayment{}, ErrHGConflict
	}
	result.PaymentID = util.GenerateBusinessID("SIMPAY")
	result.Status = 1
	requestID := "SIMPAY_" + orderID
	sum := sha256.Sum256([]byte(orderID))
	_, err = tx.ExecContext(ctx, q.HGCommerceSimPaymentInsertSQL, result.PaymentID, orderID, user, requestID, hex.EncodeToString(sum[:]), total, merchant, expiry)
	if err != nil {
		return HGSimulatedPayment{}, hgDBError(err)
	}
	return result, tx.Commit()
}

// HGApplySimulatedEvent 只处理经过专用密钥签名的成功模拟收款，锁同一订单行与过期关闭竞争。
// 已取消/已过期的收款在同一事务留存付款与模拟原路退款事实；绝不恢复履约或重占库存。
func (s *HGOrderStore) HGApplySimulatedEvent(ctx context.Context, secret, raw []byte, signature string) (HGSimulatedReceipt, error) {
	if !config.IsCommerceDebugTradingEnabled() {
		return HGSimulatedReceipt{}, ErrHGPaymentUnavailable
	}
	if len(raw) == 0 || len(raw) > 4<<10 {
		return HGSimulatedReceipt{}, ErrHGInvalid
	}
	if !hgVerifySimulated(secret, raw, signature) {
		return HGSimulatedReceipt{}, ErrHGForbidden
	}
	var event HGSimulatedEvent
	if err := json.Unmarshal(raw, &event); err != nil {
		return HGSimulatedReceipt{}, ErrHGInvalid
	}
	if !hgID(event.EventID) || !hgID(event.PaymentID) || !hgID(event.OrderID) || !hgID(event.ExternalPaymentID) || event.Currency != "CNY" || event.Status != "SUCCESS" || event.AmountCent <= 0 {
		return HGSimulatedReceipt{}, ErrHGInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return HGSimulatedReceipt{}, err
	}
	defer tx.Rollback()
	var owner, product, sku, merchant string
	var qty uint32
	var status uint8
	var expiry time.Time
	var amount int64
	if err = tx.QueryRowContext(ctx, q.HGCommerceSimOrderLockSQL, event.OrderID).Scan(&owner, &product, &sku, &qty, &status, &expiry, &amount, &merchant); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return HGSimulatedReceipt{}, ErrHGForbidden
		}
		return HGSimulatedReceipt{}, err
	}
	var savedHash []byte
	var result HGSimulatedReceipt
	hash := sha256.Sum256(raw)
	err = tx.QueryRowContext(ctx, q.HGCommerceSimEventReadSQL, event.EventID).Scan(&savedHash, &result.Result)
	if err == nil {
		if subtle.ConstantTimeCompare(savedHash, hash[:]) != 1 {
			return HGSimulatedReceipt{}, ErrHGConflict
		}
		return result, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return HGSimulatedReceipt{}, err
	}
	if amount != event.AmountCent {
		return HGSimulatedReceipt{}, ErrHGConflict
	}
	var paymentID, payer string
	var paymentAmount int64
	var paymentStatus uint8
	if err = tx.QueryRowContext(ctx, q.HGCommerceSimPaymentGetSQL, event.OrderID).Scan(&paymentID, &payer, &paymentAmount, &paymentStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return HGSimulatedReceipt{}, ErrHGConflict
		}
		return HGSimulatedReceipt{}, err
	}
	if paymentID != event.PaymentID || payer != owner || paymentAmount != amount || paymentStatus != 1 {
		return HGSimulatedReceipt{}, ErrHGConflict
	}
	if err = hgExactlyOne(tx.ExecContext(ctx, q.HGCommerceSimPaymentSuccessSQL, event.ExternalPaymentID, paymentID, event.OrderID, amount)); err != nil {
		return HGSimulatedReceipt{}, err
	}
	late := status == 5 || (status == 1 && !expiry.After(time.Now().UTC()))
	switch {
	case status == 1 && !late:
		result.Result = "PAID"
		if err = hgExactlyOne(tx.ExecContext(ctx, q.HGCommerceSimReservationSellSQL, event.OrderID)); err != nil {
			return HGSimulatedReceipt{}, err
		}
		if err = hgExactlyOne(tx.ExecContext(ctx, q.HGCommerceSimStockSellSQL, qty, qty, product, sku, qty)); err != nil {
			return HGSimulatedReceipt{}, err
		}
		if err = hgExactlyOne(tx.ExecContext(ctx, q.HGCommerceSimOrderPaidSQL, event.OrderID)); err != nil {
			return HGSimulatedReceipt{}, err
		}
		if err = hgExactlyOne(tx.ExecContext(ctx, q.HGCommerceSimTradePaidSQL, event.OrderID)); err != nil {
			return HGSimulatedReceipt{}, err
		}
	case late:
		result.Result = "REFUNDED_LATE"
		if status == 1 {
			result.Result = "REFUNDED_EXPIRED"
			if err = hgExactlyOne(tx.ExecContext(ctx, q.HGCommerceReservationReleaseSQL, event.OrderID)); err != nil {
				return HGSimulatedReceipt{}, err
			}
			if err = hgExactlyOne(tx.ExecContext(ctx, q.HGCommerceStockReleaseSQL, qty, qty, product, sku, qty)); err != nil {
				return HGSimulatedReceipt{}, err
			}
			if err = hgExactlyOne(tx.ExecContext(ctx, q.HGCommerceOrderCloseSQL, event.OrderID)); err != nil {
				return HGSimulatedReceipt{}, err
			}
			if err = hgExactlyOne(tx.ExecContext(ctx, q.HGCommerceTradeCloseSQL, event.OrderID)); err != nil {
				return HGSimulatedReceipt{}, err
			}
		}
		if err = hgSimRefund(ctx, tx, event.OrderID, paymentID, owner, amount, "模拟迟到收款原路退款"); err != nil {
			return HGSimulatedReceipt{}, err
		}
	default:
		return HGSimulatedReceipt{}, ErrHGConflict
	}
	if _, err = tx.ExecContext(ctx, q.HGCommerceSimEventInsertSQL, event.EventID, paymentID, event.OrderID, event.ExternalPaymentID, amount, hash[:], result.Result); err != nil {
		return HGSimulatedReceipt{}, hgDBError(err)
	}
	return result, tx.Commit()
}

func hgSimRefund(ctx context.Context, tx *sql.Tx, orderID, paymentID, user string, amount int64, reason string) error {
	key := "SIMREF_" + orderID
	sum := sha256.Sum256([]byte(key))
	_, err := tx.ExecContext(ctx, q.HGCommerceSimRefundCreateSQL, util.GenerateBusinessID("SIMREF"), orderID, paymentID, user, key, hex.EncodeToString(sum[:]), amount, reason)
	return hgDBError(err)
}

// HGRefundSimulated 模拟已到账但尚未发货的原路退款；不无故回补已售实物库存。
func (s *HGOrderStore) HGRefundSimulated(ctx context.Context, user, orderID string) error {
	if !config.IsCommerceDebugTradingEnabled() {
		return ErrHGPaymentUnavailable
	}
	if user == "" || !hgID(orderID) {
		return ErrHGInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var owner, product, sku, merchant string
	var qty uint32
	var status uint8
	var expiry time.Time
	var amount int64
	if err = tx.QueryRowContext(ctx, q.HGCommerceSimOrderLockSQL, orderID).Scan(&owner, &product, &sku, &qty, &status, &expiry, &amount, &merchant); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrHGForbidden
		}
		return err
	}
	if owner != user {
		return ErrHGForbidden
	}
	if status == 6 {
		return nil
	}
	if status != 2 {
		return ErrHGConflict
	}
	var paymentID, payer string
	var paidAmount int64
	var paymentStatus uint8
	if err = tx.QueryRowContext(ctx, q.HGCommerceSimPaymentGetSQL, orderID).Scan(&paymentID, &payer, &paidAmount, &paymentStatus); err != nil {
		return err
	}
	if payer != user || paidAmount != amount || paymentStatus != 2 {
		return ErrHGConflict
	}
	if err = hgSimRefund(ctx, tx, orderID, paymentID, user, amount, "模拟未发货退款"); err != nil {
		return err
	}
	if err = hgExactlyOne(tx.ExecContext(ctx, q.HGCommerceSimOrderRefundSQL, orderID)); err != nil {
		return err
	}
	if err = hgExactlyOne(tx.ExecContext(ctx, q.HGCommerceSimTradeRefundSQL, orderID)); err != nil {
		return err
	}
	return tx.Commit()
}

// HGShipSimulated 只在严格 debug 路由调用，锁订单并检查 SIMULATED 支付事实，退款后不发货。
func (s *HGOrderStore) HGShipSimulated(ctx context.Context, user string, r HGShipRequest) error {
	if !config.IsCommerceDebugTradingEnabled() {
		return ErrHGPaymentUnavailable
	}
	if user == "" || !hgID(r.OrderID) || !hgText(r.Carrier, 64, true) || len(r.TrackingNumber) < 1 || len(r.TrackingNumber) > 128 {
		return ErrHGInvalid
	}
	for _, c := range r.TrackingNumber {
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '.' || c == '-' || c == '_') {
			return ErrHGInvalid
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var status uint8
	if err = tx.QueryRowContext(ctx, q.HGCommerceSimShipLockSQL, user, r.OrderID).Scan(&status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrHGForbidden
		}
		return err
	}
	if status != 2 {
		return ErrHGConflict
	}
	var proof int
	if err = tx.QueryRowContext(ctx, q.HGCommerceSimShipProofSQL, r.OrderID).Scan(&proof); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrHGForbidden
		}
		return err
	}
	if err = hgExactlyOne(tx.ExecContext(ctx, q.HGCommerceShipUpdateSQL, r.Carrier, r.TrackingNumber, r.OrderID)); err != nil {
		return err
	}
	if err = hgExactlyOne(tx.ExecContext(ctx, q.HGCommerceShipTradeSQL, r.OrderID)); err != nil {
		return err
	}
	return tx.Commit()
}

// HGReconcileSimulated 按订单点查六种权威事实，只报告漂移，不修改资金、预占或库存。
func (s *HGOrderStore) HGReconcileSimulated(ctx context.Context, id string) (HGReconciliation, error) {
	if !hgID(id) {
		return HGReconciliation{}, ErrHGInvalid
	}
	result := HGReconciliation{OrderID: id}
	err := s.db.QueryRowContext(ctx, q.HGCommerceSimReconcileSQL, id).Scan(&result.OrderStatus, &result.TradeStatus, &result.PaymentStatus, &result.ReservationStatus, &result.EventResult, &result.RefundStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return HGReconciliation{}, ErrHGForbidden
	}
	if err != nil {
		return HGReconciliation{}, err
	}
	switch result.OrderStatus {
	case 1:
		result.Drift = result.TradeStatus != 1 || result.ReservationStatus != 1 || result.EventResult != "" || result.RefundStatus != 0 || result.PaymentStatus > 1
	case 2:
		result.Drift = result.TradeStatus != 2 || result.ReservationStatus != 3 || result.PaymentStatus != 2 || result.EventResult != "PAID" || result.RefundStatus != 0
	case 3:
		result.Drift = result.TradeStatus != 3 || result.ReservationStatus != 3 || result.PaymentStatus != 2 || result.EventResult != "PAID" || result.RefundStatus != 0
	case 5:
		result.Drift = result.TradeStatus != 5 || result.ReservationStatus != 2 || result.RefundStatus != 0 && result.RefundStatus != 2 || result.PaymentStatus == 2 && (result.RefundStatus != 2 || result.EventResult != "REFUNDED_LATE" && result.EventResult != "REFUNDED_EXPIRED") || result.PaymentStatus != 2 && result.EventResult != ""
	case 6:
		result.Drift = result.TradeStatus != 7 || result.ReservationStatus != 3 || result.PaymentStatus != 2 || result.EventResult != "PAID" || result.RefundStatus != 2
	default:
		result.Drift = true
	}
	return result, nil
}
