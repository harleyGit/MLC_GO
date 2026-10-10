package refund

import (
	"context"
	"time"
)

// HGRefundOrder 是 refund_order 的数据库模型；金额为整数最小单位。
type HGRefundOrder struct {
	ID             uint64     // 数据库内部自增主键。
	RefundOrderID  string     // 退款订单业务标识。
	TradeOrderID   string     // 原交易订单业务标识。
	PaymentOrderID *string    // 原支付单业务标识，可为空。
	UserID         string     // 退款用户业务标识，兼容 users.user_id。
	RequestID      string     // 退款请求幂等标识，与 UserID 联合唯一。
	RequestHash    string     // 请求规范化内容 SHA-256，用于幂等冲突校验。
	Currency       string     // 退款币种，支持 CNY 与 MLC_COIN。
	Amount         int64      // 退款金额，整数最小单位。
	RefundStatus   int8       // 退款状态，取值见 refund_order.refund_status。
	Reason         *string    // 退款原因，可为空。
	ExpireAt       *time.Time // 退款处理截止时间，UTC 毫秒精度，可为空。
	CompletedAt    *time.Time // 退款完成时间，UTC 毫秒精度，可为空。
	CreatedAt      time.Time  // 创建时间，UTC 毫秒精度。
	UpdatedAt      time.Time  // 更新时间，UTC 毫秒精度。
}

// HGRefundOrderRepository 是退款订单持久化契约，当前不执行账户冲正或外部退款调用。
// 同用户同 RequestID 重放须比较规范化请求 SHA-256 RequestHash，内容不同返回冲突；未找到返回可识别错误。
// 创建须锁定原交易/支付，校验用户、同币种、成功实收及累计已退和处理中金额，防止并发超退。
// PaymentOrderID 指向 payment_order.payment_id；允许暂未分配的申请为空，但执行退款前必须绑定原成功支付。
type HGRefundOrderRepository interface {
	FindByRequest(ctx context.Context, userID string, requestID string) (*HGRefundOrder, error)
	Create(ctx context.Context, order *HGRefundOrder) error
}
