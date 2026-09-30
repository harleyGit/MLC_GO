package account

import "context"

// HGAccountReader 是账户读取能力，避免 account 反向依赖 coin 的记账实现。
// 账户写入必须继续通过 coin repository 的现有幂等事务引擎完成。
type HGAccountReader interface {
	GetAccount(ctx context.Context, userID string) (HGAccount, error)
	ListLedger(ctx context.Context, userID string, limit int) ([]HGAccountLedger, error)
}
