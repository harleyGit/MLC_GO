package accountrepository

import (
	"MLC_GO/internal/modules/account"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"fmt"
)

// HGRepository 只负责读取 account/account_ledger，写入仍由 coin repository 统一完成。
type HGRepository struct {
	db *sql.DB // db 是调用方管理生命周期的数据库连接池。
}

// NewHGRepository 创建账户只读适配器。
func NewHGRepository(db *sql.DB) *HGRepository { return &HGRepository{db: db} }

// GetAccount 读取账户快照，不对缺失账户假设余额为零。
func (r *HGRepository) GetAccount(ctx context.Context, userID string) (account.HGAccount, error) {
	if r == nil || r.db == nil {
		return account.HGAccount{}, fmt.Errorf("账户数据库不能为空")
	}
	var item account.HGAccount
	if err := r.db.QueryRowContext(ctx, SQLQueriesPackage.HGSelectAccountSQL, userID).Scan(
		&item.UserID, &item.Balance, &item.FrozenBalance, &item.Currency, &item.AccountType, &item.Version, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return account.HGAccount{}, fmt.Errorf("读取账户失败: %w", err)
	}
	return item, nil
}

// ListLedger 读取固定上限流水，避免账户审计接口产生无界结果。
func (r *HGRepository) ListLedger(ctx context.Context, userID string, limit int) ([]account.HGAccountLedger, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("账户数据库不能为空")
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := r.db.QueryContext(ctx, SQLQueriesPackage.HGSelectAccountLedgerSQL, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("读取账户流水失败: %w", err)
	}
	defer rows.Close()
	items := make([]account.HGAccountLedger, 0, limit)
	for rows.Next() {
		var item account.HGAccountLedger
		var reference sql.NullInt64
		if err := rows.Scan(&item.ID, &item.UserID, &item.RequestID, &item.Operation, &item.Amount, &item.SignedDelta, &item.BalanceAfter, &item.BalanceBefore, &item.Reason, &item.BusinessType, &item.BusinessKey, &reference, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("解析账户流水失败: %w", err)
		}
		if reference.Valid && reference.Int64 > 0 {
			item.ReferenceTransactionID = uint64(reference.Int64)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历账户流水失败: %w", err)
	}
	return items, nil
}
