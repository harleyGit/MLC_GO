package recharge

import (
	"context"
	"time"
)

// HGPaymentRechargeOrder 是 payment_recharge_order 的数据库模型；total_coin 必须等于 coin_amount 加 bonus_coin。
type HGPaymentRechargeOrder struct {
	ID               uint64     // 数据库内部自增主键。
	RechargeOrderID  string     // 充值订单业务标识。
	TradeOrderID     string     // 关联交易订单业务标识。
	UserID           string     // 充值用户业务标识，兼容 users.user_id。
	RequestID        string     // 创建请求幂等标识，与 UserID 联合唯一。
	RequestHash      string     // 请求规范化内容 SHA-256，用于幂等冲突校验。
	SKUID            string     // 复用 payment_recharge_sku 的档位标识，数据库为 VARCHAR(64) utf8mb4_general_ci。
	SKUSnapshotTitle string     // 充值档位标题快照。
	Currency         string     // 充值支付币种仅允许 CNY，禁止使用平台币自充。
	PayAmount        int64      // 实际支付人民币金额，单位分。
	CoinAmount       int64      // 基础平台币数量。
	BonusCoin        int64      // 赠送平台币数量。
	TotalCoin        int64      // 平台币总数量，等于基础币加赠币。
	OrderStatus      int8       // 充值订单状态，取值见 payment_recharge_order.order_status。
	PaymentStatus    int8       // 充值支付状态，取值见 payment_recharge_order.payment_status。
	ExpireAt         time.Time  // 支付截止时间，UTC 毫秒精度。
	PaidAt           *time.Time // 支付完成时间，UTC 毫秒精度，可为空。
	CompletedAt      *time.Time // 充值完成时间，UTC 毫秒精度，可为空。
	CancelledAt      *time.Time // 取消时间，UTC 毫秒精度，可为空。
	CreatedAt        time.Time  // 创建时间，UTC 毫秒精度。
	UpdatedAt        time.Time  // 更新时间，UTC 毫秒精度。
}

// HGRechargeOrderRepository 是充值订单持久化契约，当前未接入账户入账或支付渠道。
// 实现须绑定与 trade 相同的短事务；校验 RECHARGE 类型、用户、币种和应付金额，不能单独提交孤立充值单。
// 充值档位、交易单、充值单及支付尝试必须均为 CNY，禁止平台币支付充值；入账币种为 MLC_COIN。
// 同用户同 RequestID 重放必须比较规范化请求 SHA-256 RequestHash，内容不同返回冲突；未找到返回可识别错误。
// SKU 复用既有 payment_recharge_sku：按原排序规则查询后复制规范 ID 和价格快照，不将 ascii_bin 强加到旧表查询。
// 旧档位无符号金额转换为 int64 前须检查范围；基础币、赠币单位沿用既有 coin 语义，不假设人民币兑换率。
// 支付确认与 outbox_event 必须原子提交，账户入账仍需独立幂等确认；这些流程当前均未实现。
type HGRechargeOrderRepository interface {
	FindByRequest(ctx context.Context, userID string, requestID string) (*HGPaymentRechargeOrder, error)
	Create(ctx context.Context, order *HGPaymentRechargeOrder) error
}
