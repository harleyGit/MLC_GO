package CoinModelPackage

import (
	"MLC_GO/internal/events"
	"time"
)

// HGOperation 是固定、低基数的资产变更类型，同时用于流水审计和 Prometheus 标签。
// 新增枚举时必须同步检查数据库约束、事件协议、指标输出和历史数据兼容性。
type HGOperation string

const (
	HGOperationInitialize HGOperation = "initialize"
	HGOperationRecharge   HGOperation = "recharge"
	HGOperationGrant      HGOperation = "grant"
	HGOperationDebit      HGOperation = "debit"
	HGOperationRefund     HGOperation = "refund"
	HGOperationExpire     HGOperation = "expire"
	HGOperationCorrection HGOperation = "correction"
)

// HGCommand 是进入权威资产事务的内部命令。
//
// RequestID 必须由可信调用方生成并在重试时保持稳定；Repository 会把命令内容哈希后与 RequestID
// 一起持久化，相同 RequestID 的不同业务参数会被判定为幂等冲突。Event 若非空，会和资产变化在同一个
// MySQL 事务中写入 Outbox。BusinessLimit 用于视频投币等有累计上限的业务，不是钱包总额限制。
type HGCommand struct {
	Operation              HGOperation
	UserID                 string
	RequestID              string
	Amount                 uint64
	SignedDelta            int64
	Reason                 string
	BusinessType           string
	BusinessKey            string
	BusinessLimit          uint64
	ReferenceTransactionID uint64
	ExpiresAt              *time.Time
	Event                  events.DomainEvent
}

// HGMutationResult 返回权威事务结果。
// Committed=false 且 error=nil 表示命中了已完成幂等请求，没有发生第二次资产变化。
type HGMutationResult struct {
	Committed     bool
	TransactionID uint64
	BalanceAfter  uint64
}

// HGUserCursor 是钱包初始化任务使用的 users.id keyset 游标项，禁止用 OFFSET 做历史用户深分页。
type HGUserCursor struct {
	ID     uint64
	UserID string
}

// HGWalletDrift 报告钱包快照与可用 lot 总额的差异，但不会自动修改任何资产数据。
// 修复必须通过带稳定 RequestID 和审计原因的 correction 流水完成。
type HGWalletDrift struct {
	UserID        string
	WalletBalance uint64
	LotBalance    uint64
}
