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

type hgConsolidationLot struct {
	id, amount uint64
	expiresAt  sql.NullTime
}

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

func (r *HGRepository) hgLoadJobCheckpoint(ctx context.Context, query string, name string) (uint64, error) {
	var cursor uint64
	err := r.db.QueryRowContext(ctx, query).Scan(&cursor)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("load %s checkpoint: %w", name, err)
	}
	return cursor, nil
}

func (r *HGRepository) hgSaveJobCheckpoint(ctx context.Context, query string, checkpoint uint64, name string) error {
	if _, err := r.db.ExecContext(ctx, query, checkpoint); err != nil {
		return fmt.Errorf("save %s checkpoint: %w", name, err)
	}
	return nil
}

func (r *HGRepository) LoadReconciliationCheckpoint(ctx context.Context) (uint64, error) {
	return r.hgLoadJobCheckpoint(ctx, SQLQueriesPackage.SelectCoinReconciliationCheckpointSQL, "coin reconciliation")
}

func (r *HGRepository) SaveReconciliationCheckpoint(ctx context.Context, checkpoint uint64) error {
	return r.hgSaveJobCheckpoint(ctx, SQLQueriesPackage.UpsertCoinReconciliationCheckpointSQL, checkpoint, "coin reconciliation")
}

func (r *HGRepository) LoadConsolidationCheckpoint(ctx context.Context) (uint64, error) {
	return r.hgLoadJobCheckpoint(ctx, SQLQueriesPackage.SelectCoinConsolidationCheckpointSQL, "coin consolidation")
}

func (r *HGRepository) SaveConsolidationCheckpoint(ctx context.Context, checkpoint uint64) error {
	return r.hgSaveJobCheckpoint(ctx, SQLQueriesPackage.UpsertCoinConsolidationCheckpointSQL, checkpoint, "coin consolidation")
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

// ConsolidateBatch 使用 users.id keyset 发现固定数量钱包，每个钱包使用 wallet-first 短事务合并同 expiry 的小 lot。
func (r *HGRepository) ConsolidateBatch(ctx context.Context, cursor uint64, userLimit int, sourceLimit int, maxLotAmount uint64) (int, uint64, error) {
	rows, err := r.db.QueryContext(ctx, SQLQueriesPackage.SelectCoinConsolidationUsersSQL, cursor, maxLotAmount, userLimit)
	if err != nil {
		return 0, cursor, fmt.Errorf("select coin consolidation users: %w", err)
	}
	type candidate struct {
		id     uint64
		userID string
	}
	candidates := make([]candidate, 0, userLimit)
	for rows.Next() {
		var item candidate
		if err := rows.Scan(&item.id, &item.userID); err != nil {
			rows.Close()
			return 0, cursor, fmt.Errorf("scan coin consolidation user: %w", err)
		}
		candidates = append(candidates, item)
	}
	if err := rows.Close(); err != nil {
		return 0, cursor, fmt.Errorf("close coin consolidation users: %w", err)
	}
	processed := 0
	next := cursor
	for _, item := range candidates {
		committed, consolidateErr := r.hgConsolidateWallet(ctx, item.userID, sourceLimit, maxLotAmount)
		if consolidateErr != nil {
			return processed, cursor, consolidateErr
		}
		if committed {
			processed++
		}
		next = item.id
	}
	if len(candidates) < userLimit {
		next = 0
	}
	return processed, next, nil
}

func (r *HGRepository) hgConsolidateWallet(ctx context.Context, userID string, sourceLimit int, maxLotAmount uint64) (bool, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return false, fmt.Errorf("begin coin consolidation: %w", err)
	}
	defer tx.Rollback()
	var balance uint64
	if err := tx.QueryRowContext(ctx, SQLQueriesPackage.SelectCoinWalletForUpdateSQL, userID).Scan(&balance); err != nil {
		return false, fmt.Errorf("lock consolidation wallet: %w", err)
	}
	rows, err := tx.QueryContext(ctx, SQLQueriesPackage.SelectCoinLotsForConsolidationSQL, userID, maxLotAmount, sourceLimit)
	if err != nil {
		return false, fmt.Errorf("select consolidation lots: %w", err)
	}
	lots := make([]hgConsolidationLot, 0, sourceLimit)
	for rows.Next() {
		var item hgConsolidationLot
		if err := rows.Scan(&item.id, &item.amount, &item.expiresAt); err != nil {
			rows.Close()
			return false, fmt.Errorf("scan consolidation lot: %w", err)
		}
		lots = append(lots, item)
	}
	if err := rows.Close(); err != nil {
		return false, fmt.Errorf("close consolidation lots: %w", err)
	}
	if len(lots) < 2 {
		return false, nil
	}
	group := hgSelectConsolidationGroup(lots)
	if len(group) < 2 {
		return false, nil
	}
	var total uint64
	for _, item := range group {
		if total > ^uint64(0)-item.amount {
			return false, fmt.Errorf("coin consolidation amount overflow")
		}
		total += item.amount
	}
	requestID := "consolidate:lot:" + strconv.FormatUint(group[0].id, 10) + ":" + strconv.FormatUint(group[len(group)-1].id, 10)
	command := CoinModelPackage.HGCommand{Operation: CoinModelPackage.HGOperationConsolidate, UserID: userID, RequestID: requestID, Amount: total, Reason: "lot_consolidation"}
	insert, err := tx.ExecContext(ctx, SQLQueriesPackage.InsertCoinRequestSQL, userID, requestID, command.Operation, hgCommandHash(command))
	if err != nil {
		return false, fmt.Errorf("insert consolidation request: %w", err)
	}
	inserted, err := insert.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("read consolidation request: %w", err)
	}
	if inserted == 0 {
		return false, nil
	}
	mutation, err := r.hgRecordMutation(ctx, tx, command, balance)
	if err != nil {
		return false, err
	}
	var expiresAt any
	if group[0].expiresAt.Valid {
		expiresAt = group[0].expiresAt.Time
	}
	result, err := tx.ExecContext(ctx, SQLQueriesPackage.InsertCoinLotSQL, userID, mutation.TransactionID, total, total, expiresAt)
	if err != nil {
		return false, fmt.Errorf("insert consolidation target lot: %w", err)
	}
	targetLotID, err := result.LastInsertId()
	if err != nil {
		return false, fmt.Errorf("read consolidation target lot: %w", err)
	}
	for _, item := range group {
		if _, err := tx.ExecContext(ctx, SQLQueriesPackage.UpdateCoinLotRemainingSQL, uint64(0), item.id); err != nil {
			return false, fmt.Errorf("zero consolidation source lot: %w", err)
		}
		if _, err := tx.ExecContext(ctx, SQLQueriesPackage.InsertCoinAllocationSQL, mutation.TransactionID, item.id, item.amount, "consolidate_source"); err != nil {
			return false, fmt.Errorf("insert consolidation source allocation: %w", err)
		}
		if _, err := tx.ExecContext(ctx, SQLQueriesPackage.InsertCoinConsolidationLinkSQL, mutation.TransactionID, item.id, uint64(targetLotID), item.amount); err != nil {
			return false, fmt.Errorf("insert consolidation link: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, SQLQueriesPackage.InsertCoinAllocationSQL, mutation.TransactionID, uint64(targetLotID), total, "consolidate_target"); err != nil {
		return false, fmt.Errorf("insert consolidation target allocation: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit coin consolidation: %w", err)
	}
	return true, nil
}

func hgSelectConsolidationGroup(lots []hgConsolidationLot) []hgConsolidationLot {
	for start := 0; start < len(lots)-1; start++ {
		group := []hgConsolidationLot{lots[start]}
		for next := start + 1; next < len(lots); next++ {
			if lots[next].expiresAt.Valid == lots[start].expiresAt.Valid && (!lots[next].expiresAt.Valid || lots[next].expiresAt.Time.Equal(lots[start].expiresAt.Time)) {
				group = append(group, lots[next])
			}
		}
		if len(group) >= 2 {
			return group
		}
	}
	return nil
}
