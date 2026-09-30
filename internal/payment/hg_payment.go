package payment

import (
	"context"
	"time"
)

// HGPaymentOrder 是 payment_order 的数据库模型；一条记录代表一次支付尝试。
type HGPaymentOrder struct {
	ID                uint64     // 数据库内部自增主键。
	PaymentID         string     // 支付尝试业务标识，每次重复尝试可不同。
	TradeOrderID      string     // 关联交易订单业务标识。
	UserID            string     // 支付用户业务标识，兼容 users.user_id。
	RequestID         string     // 支付尝试请求幂等标识，与用户联合唯一；新尝试使用新值。
	RequestHash       string     // 规范化请求 SHA-256，重放必须比较哈希并拒绝内容冲突。
	PaymentType       string     // 支付方式，例如 COIN。
	Currency          string     // 支付币种，支持 CNY 与 MLC_COIN。
	Amount            int64      // 支付金额，整数最小单位。
	PaymentStatus     int8       // 支付状态，取值见 payment_order.payment_status。
	ClientType        *string    // 发起支付的客户端类型，可为空。
	MerchantID        *string    // 外部商户业务标识，可为空。
	Channel           *string    // 外部支付渠道标识，可为空。
	ExternalPaymentID *string    // 外部支付单号，可为空；按商户和渠道唯一。
	ExpireAt          time.Time  // 支付尝试截止时间，UTC 毫秒精度。
	PaidAt            *time.Time // 支付成功时间，UTC 毫秒精度，可为空。
	CreatedAt         time.Time  // 创建时间，UTC 毫秒精度。
	UpdatedAt         time.Time  // 更新时间，UTC 毫秒精度。
}

// HGPaymentOrderRepository 是支付单持久化契约，当前不包含微信、支付宝或其他 SDK 适配。
// Create 必须校验订单归属、币种及全额应付金额；只支持同币种全额重复尝试，不支持拆单或混合币种。
// 同用户同 RequestID 重放须比较规范化请求的 SHA-256 RequestHash，不同则返回冲突，不能只依赖唯一键。
// 多次尝试成功、超时后迟到成功必须由后续编排锁定交易单并处理对账/退款，不可重复履约或忽略实收。
// FindByPaymentID 未找到须返回可识别错误；状态更新及 outbox_event 尚未接入，禁止把建单当作支付成功。
type HGPaymentOrderRepository interface {
	FindByPaymentID(ctx context.Context, paymentID string) (*HGPaymentOrder, error)
	Create(ctx context.Context, order *HGPaymentOrder) error
}
