package repository

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// HGErrUserHasDependencies 表示用户存在业务记录、历史审计或非零余额，不允许物理删除。
var HGErrUserHasDependencies = errors.New("用户存在关联记录，不能删除")

// HGErrUserDeleteCommitUncertain 表示提交结果不明，调用方不得移除认证隔离标记。
var HGErrUserDeleteCommitUncertain = errors.New("用户删除事务提交结果待核实")

// HGCanonicalDeleteUserID 返回 MySQL 中真实的业务 ID，后续 Redis 隔离和 SQL 必须使用同一值。
func (r *UserRepo) HGCanonicalDeleteUserID(ctx context.Context, userID string) (string, error) {
	queryCtx, cancel := context.WithTimeout(ctx, userRepoQueryTimeout)
	defer cancel()
	var canonicalID string
	err := r.QueryRow(queryCtx, SQLQueriesPackage.HGUserDeleteCanonicalIDSQL, userID).Scan(&canonicalID)
	return canonicalID, err
}

// HGDeleteUnusedUser 在一个短事务内锁定账号并逐项检查业务归属，空钱包可随账号清理。
// 查询失败时一律回滚；调用者须在事务前阻止新会话并处理 Redis 中已有会话。
func (r *UserRepo) HGDeleteUnusedUser(ctx context.Context, userID, actorID string) error {
	queryCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	tx, err := r.BeginTx(queryCtx, nil)
	if err != nil {
		return fmt.Errorf("开启用户删除事务失败: %w", err)
	}
	defer tx.Rollback()

	var id int64
	if err = tx.QueryRowContext(queryCtx, SQLQueriesPackage.HGUserDeleteLockSQL, userID).Scan(&id); err != nil {
		return err
	}
	for _, query := range SQLQueriesPackage.HGUserDeleteDependencies {
		var found int
		err = tx.QueryRowContext(queryCtx, query, userID).Scan(&found)
		if err == nil {
			return HGErrUserHasDependencies
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("核查用户关联记录失败: %w", err)
		}
	}
	for _, query := range SQLQueriesPackage.HGUserDeleteNumericDependencies {
		var found int
		err = tx.QueryRowContext(queryCtx, query, id).Scan(&found)
		if err == nil {
			return HGErrUserHasDependencies
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("核查历史用户关联记录失败: %w", err)
		}
	}
	var balance, frozenBalance uint64
	err = tx.QueryRowContext(queryCtx, SQLQueriesPackage.HGUserDeleteWalletSQL, userID).Scan(&balance, &frozenBalance)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("核查用户钱包失败: %w", err)
	}
	if err == nil {
		if balance != 0 || frozenBalance != 0 {
			return HGErrUserHasDependencies
		}
		if _, err = tx.ExecContext(queryCtx, SQLQueriesPackage.HGUserDeleteWalletRowSQL, userID); err != nil {
			return fmt.Errorf("清理空钱包失败: %w", err)
		}
	}
	result, err := tx.ExecContext(queryCtx, SQLQueriesPackage.HGUserDeleteAccountSQL, userID)
	if err != nil {
		return fmt.Errorf("删除用户失败: %w", err)
	}
	if err = ensureRowsAffected(result); err != nil {
		return err
	}
	if _, err = tx.ExecContext(queryCtx, SQLQueriesPackage.HGUserDeleteAuditSQL, actorID, userID); err != nil {
		return fmt.Errorf("写入用户删除审计失败: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("%w: %w", HGErrUserDeleteCommitUncertain, err)
	}
	return nil
}
