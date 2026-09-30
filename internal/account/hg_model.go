package account

import "time"

// HGAccount 是账户权威表的只读模型；保留 user_id 主键，一个用户仅一个 MLC_COIN 账户。
type HGAccount struct {
	// UserID 是账户所属用户标识，对应 account.user_id 主键。
	UserID string
	// Balance 是可用余额，单位为最小 MLC_COIN；不能为空且不为负。
	Balance uint64
	// FrozenBalance 是冻结余额，单位为最小 MLC_COIN；当前版本固定读取但不开放冻结业务。
	FrozenBalance uint64
	// Currency 是账户币种；当前仅支持 MLC_COIN，不支持多币种。
	Currency string
	// AccountType 是账户类型；当前值为 user。
	AccountType string
	// Version 是余额变更版本号；余额每次更新递增。
	Version uint64
	// CreatedAt 是账户创建时间。
	CreatedAt time.Time
	// UpdatedAt 是账户最近更新时间。
	UpdatedAt time.Time
}

// HGAccountLedger 是账户流水的只读审计模型，不提供绕过 coin 记账引擎的写入入口。
type HGAccountLedger struct {
	// ID 是账户流水主键，并保留原 coin_asset_transactions 关联语义。
	ID uint64
	// UserID 是流水所属用户标识。
	UserID string
	// RequestID 是幂等请求标识。
	RequestID string
	// Operation 是资产操作类型，例如 debit、refund、grant。
	Operation string
	// Amount 是本次操作绝对数量，单位为最小 MLC_COIN。
	Amount uint64
	// SignedDelta 是带符号余额变化量，单位为最小 MLC_COIN。
	SignedDelta int64
	// BalanceAfter 是操作后的余额，单位为最小 MLC_COIN。
	BalanceAfter uint64
	// BalanceBefore 是数据库生成的变更前余额文本，使用 DECIMAL 保留历史兼容性。
	BalanceBefore string
	// Reason 是业务原因。
	Reason string
	// BusinessType 是业务域类型。
	BusinessType string
	// BusinessKey 是业务关联键。
	BusinessKey string
	// ReferenceTransactionID 是关联原流水 ID；无关联时为空。
	ReferenceTransactionID uint64
	// CreatedAt 是流水创建时间。
	CreatedAt time.Time
}
