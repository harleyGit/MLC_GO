package trade

import (
	"context"
	"time"
)

// HGTradeOrder 是 trade_order 的数据库模型；金额均为币种对应的整数最小单位。
type HGTradeOrder struct {
	ID             uint64     // 数据库内部自增主键。
	TradeOrderID   string     // 交易订单业务标识。
	UserID         string     // 下单用户业务标识，兼容 users.user_id。
	RequestID      string     // 创建请求幂等标识，与 UserID 联合唯一。
	RequestHash    string     // 请求规范化内容 SHA-256，用于幂等冲突校验。
	OrderType      string     // 订单类型：RECHARGE、MEMBERSHIP 或 PRODUCT。
	Title          string     // 下单时订单标题快照。
	Currency       string     // 订单币种，支持 CNY 与 MLC_COIN。
	TotalAmount    int64      // 商品总金额，整数最小单位。
	DiscountAmount int64      // 优惠金额，整数最小单位。
	PayableAmount  int64      // 应付金额，整数最小单位，等于总额减优惠。
	TradeStatus    int8       // 交易状态，取值见 trade_order.trade_status。
	Source         *string    // 订单来源，可为空。
	ExpireAt       *time.Time // 支付截止时间，UTC 毫秒精度，可为空。
	PaidAt         *time.Time // 支付完成时间，UTC 毫秒精度，可为空。
	CompletedAt    *time.Time // 交易完成时间，UTC 毫秒精度，可为空。
	CancelledAt    *time.Time // 取消时间，UTC 毫秒精度，可为空。
	CreatedAt      time.Time  // 创建时间，UTC 毫秒精度。
	UpdatedAt      time.Time  // 更新时间，UTC 毫秒精度。
}

// HGTradeOrderItem 是 trade_order_item 的数据库模型；quantity 必须为1至100000。
type HGTradeOrderItem struct {
	ID           uint64    // 数据库内部自增主键。
	OrderItemID  string    // 订单明细业务标识。
	TradeOrderID string    // 所属交易订单业务标识。
	LineNo       uint16    // 订单内行号1至100，与订单业务标识联合唯一，限制每单最多100行。
	UserID       string    // 下单用户业务标识，兼容 users.user_id。
	ProductID    string    // 商品业务标识快照。
	SKUID        string    // SKU 业务标识快照。
	ProductTitle string    // 商品名称快照。
	SKUTitle     *string   // SKU 名称快照，可为空。
	Quantity     uint32    // 购买数量，范围为1至100000。
	UnitAmount   int64     // 明细单价，整数最小单位。
	TotalAmount  int64     // 明细总金额，整数最小单位，等于单价乘数量。
	CreatedAt    time.Time // 创建时间，UTC 毫秒精度。
}

// HGTradeOrderRepository 是交易订单持久化契约，当前目录只定义接口，尚未接入实现。
// Create 必须在同一短事务内写入订单和1至100条明细，校验用户、币种及明细汇总；充值明细引用充值档位，不能虚造 product 记录。
// 同用户同请求标识只允许重放同一规范化请求（固定字段顺序、金额单位和版本的 SHA-256 小写十六进制）；
// 不同 RequestHash 必须返回可识别的冲突错误，唯一键冲突不可当作无条件成功。未找到返回可识别的未找到错误。
// 接口必须由事务作用域实现提供；充值聚合写入须与 recharge 共用事务，不得各自独立提交。
type HGTradeOrderRepository interface {
	FindByRequest(ctx context.Context, userID string, requestID string) (*HGTradeOrder, error)
	Create(ctx context.Context, order *HGTradeOrder, items []HGTradeOrderItem) error
}
