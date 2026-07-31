package CoinRepositoryPackage

import (
	"MLC_GO/internal/events"
	CoinEventsPackage "MLC_GO/internal/events/coin"
	CoinModelPackage "MLC_GO/internal/modules/coin/model"
	"MLC_GO/internal/outbox"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"
)

// LoadInitializerCheckpoint 读取历史钱包初始化的持久化 users.id 游标。
func (r *HGRepository) LoadInitializerCheckpoint(ctx context.Context) (uint64, error) {
	var cursor uint64
	err := r.db.QueryRowContext(ctx, SQLQueriesPackage.SelectCoinInitializerCheckpointSQL).Scan(&cursor)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("load coin initializer checkpoint: %w", err)
	}
	return cursor, nil
}

// ListUsersAfter 使用 users.id keyset 分页读取固定批次，禁止 OFFSET 深分页和无界扫描。
func (r *HGRepository) ListUsersAfter(ctx context.Context, cursor uint64, limit int) ([]CoinModelPackage.HGUserCursor, error) {
	rows, err := r.db.QueryContext(ctx, SQLQueriesPackage.SelectUsersAfterCoinCursorSQL, cursor, limit)
	if err != nil {
		return nil, fmt.Errorf("list users for coin initializer: %w", err)
	}
	defer rows.Close()
	users := make([]CoinModelPackage.HGUserCursor, 0, limit)
	for rows.Next() {
		var user CoinModelPackage.HGUserCursor
		if err := rows.Scan(&user.ID, &user.UserID); err != nil {
			return nil, fmt.Errorf("scan coin initializer user: %w", err)
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

// SaveInitializerCheckpoint 单调推进初始化游标；任务重放时 INSERT IGNORE wallet 仍保持幂等。
func (r *HGRepository) SaveInitializerCheckpoint(ctx context.Context, checkpoint uint64) error {
	_, err := r.db.ExecContext(ctx, SQLQueriesPackage.UpsertCoinInitializerCheckpointSQL, checkpoint)
	if err != nil {
		return fmt.Errorf("save coin initializer checkpoint: %w", err)
	}
	return nil
}

// ExpireBatch 通过到期索引发现固定数量候选，再让每个 lot 使用独立短事务过期。
// 钱包始终先于 lot 加锁，使在线 debit 和后台 expire 使用同一锁顺序；候选重复发现由行锁和幂等键兜底。
func (r *HGRepository) ExpireBatch(ctx context.Context, limit int, now time.Time) (int, error) {
	rows, err := r.db.QueryContext(ctx, SQLQueriesPackage.SelectExpiredCoinLotsSQL, now.UTC(), limit)
	if err != nil {
		return 0, fmt.Errorf("select expired coin lots: %w", err)
	}
	type candidate struct {
		id     uint64
		userID string
	}
	candidates := make([]candidate, 0, limit)
	for rows.Next() {
		var item candidate
		if err := rows.Scan(&item.id, &item.userID); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan expired coin lot: %w", err)
		}
		candidates = append(candidates, item)
	}
	if err := rows.Close(); err != nil {
		return 0, fmt.Errorf("close expired coin lots: %w", err)
	}
	processed := 0
	for _, item := range candidates {
		committed, expireErr := r.hgExpireLot(ctx, item.userID, item.id, now.UTC())
		if expireErr != nil {
			return processed, expireErr
		}
		if committed {
			processed++
		}
	}
	return processed, nil
}

func (r *HGRepository) hgExpireLot(ctx context.Context, userID string, lotID uint64, now time.Time) (bool, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return false, fmt.Errorf("begin coin expiration: %w", err)
	}
	defer tx.Rollback()
	var balance uint64
	if err := tx.QueryRowContext(ctx, SQLQueriesPackage.SelectCoinWalletForUpdateSQL, userID).Scan(&balance); err != nil {
		return false, fmt.Errorf("lock expiration wallet: %w", err)
	}
	var amount uint64
	if err := tx.QueryRowContext(ctx, SQLQueriesPackage.SelectExpiredCoinLotForUpdateSQL, lotID, userID, now).Scan(&amount); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("lock expired coin lot: %w", err)
	}
	if amount == 0 {
		return false, nil
	}
	if balance < amount {
		return false, fmt.Errorf("expiration wallet drift for user: %w", ErrHGInsufficientBalance)
	}
	requestID := "expire:lot:" + strconv.FormatUint(lotID, 10)
	command := CoinModelPackage.HGCommand{Operation: CoinModelPackage.HGOperationExpire, UserID: userID, RequestID: requestID, Amount: amount, Reason: "lot_expired"}
	insert, err := tx.ExecContext(ctx, SQLQueriesPackage.InsertCoinRequestSQL, userID, requestID, command.Operation, hgCommandHash(command))
	if err != nil {
		return false, fmt.Errorf("insert expiration request: %w", err)
	}
	inserted, err := insert.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("read expiration request: %w", err)
	}
	if inserted == 0 {
		return false, nil
	}
	if _, err := tx.ExecContext(ctx, SQLQueriesPackage.DebitCoinWalletSQL, amount, userID, amount); err != nil {
		return false, fmt.Errorf("debit expired coin wallet: %w", err)
	}
	mutation, err := r.hgRecordMutation(ctx, tx, command, balance-amount)
	if err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(ctx, SQLQueriesPackage.UpdateCoinLotRemainingSQL, uint64(0), lotID); err != nil {
		return false, fmt.Errorf("zero expired coin lot: %w", err)
	}
	if _, err := tx.ExecContext(ctx, SQLQueriesPackage.InsertCoinAllocationSQL, mutation.TransactionID, lotID, amount, "expire"); err != nil {
		return false, fmt.Errorf("insert expiration allocation: %w", err)
	}
	event := CoinEventsPackage.HGAssetChangedEvent{EventMeta: events.NewEventMeta(ctx), UserID: userID, Operation: string(CoinModelPackage.HGOperationExpire), Amount: amount, BalanceAfter: balance - amount}
	if err := outbox.NewRepository(r.db, r.topic).SaveTx(ctx, tx, event); err != nil {
		return false, fmt.Errorf("save expiration outbox: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit coin expiration: %w", err)
	}
	return true, nil
}

// ReconcileBatch 按 users.id 游标比较 wallet 与 active lot 总额，只报告差异，不自动修复资产。
func (r *HGRepository) ReconcileBatch(ctx context.Context, cursor uint64, limit int) ([]CoinModelPackage.HGWalletDrift, uint64, error) {
	rows, err := r.db.QueryContext(ctx, SQLQueriesPackage.SelectCoinReconciliationPageSQL, cursor, limit)
	if err != nil {
		return nil, cursor, fmt.Errorf("select coin reconciliation page: %w", err)
	}
	defer rows.Close()
	drifts := make([]CoinModelPackage.HGWalletDrift, 0)
	var next uint64 = cursor
	rowCount := 0
	for rows.Next() {
		var id uint64
		var userID string
		var wallet, lots uint64
		if err := rows.Scan(&id, &userID, &wallet, &lots); err != nil {
			return nil, cursor, fmt.Errorf("scan coin reconciliation row: %w", err)
		}
		next = id
		rowCount++
		if wallet != lots {
			drifts = append(drifts, CoinModelPackage.HGWalletDrift{UserID: userID, WalletBalance: wallet, LotBalance: lots})
		}
	}
	if rowCount < limit {
		next = 0
	}
	return drifts, next, rows.Err()
}
