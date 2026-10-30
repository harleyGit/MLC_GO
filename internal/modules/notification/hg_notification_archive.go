package notification

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"fmt"
	"time"
)

// HGArchiveBatchOffline 仅归档已软删除且到期的通知；未读/可见通知始终保留在活跃表。
// 调用者必须保存返回游标，按窗口逐步恢复；每次最多扫描 10000 个 ID、搬运 100 行，不自动运行。
func (repo *HGRepository) HGArchiveBatchOffline(ctx context.Context, after uint64, before time.Time) (uint64, int, error) {
	if repo == nil || repo.db == nil || before.IsZero() || !before.Before(time.Now().Add(-24*time.Hour)) {
		return after, 0, HGErrInvalid
	}
	upper := after + 10000
	if upper < after {
		upper = ^uint64(0) >> 1
	}
	if upper > uint64(^uint64(0)>>1) {
		upper = uint64(^uint64(0) >> 1)
	}
	if upper <= after {
		return after, 0, HGErrInvalid
	}
	tx, err := repo.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return after, 0, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, SQLQueriesPackage.HGNotificationArchiveWindowSQL, after, upper, before)
	if err != nil {
		return after, 0, err
	}
	type archived struct {
		id                                                            uint64
		eventID, user, category, title, content, targetType, targetID string
		read, deleted                                                 bool
		created                                                       time.Time
	}
	items := make([]archived, 0, 100)
	for rows.Next() {
		var item archived
		if err = rows.Scan(&item.id, &item.eventID, &item.user, &item.category, &item.title, &item.content, &item.targetType, &item.targetID, &item.read, &item.deleted, &item.created); err != nil {
			break
		}
		items = append(items, item)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return after, 0, err
	}
	for _, item := range items {
		shard := hgNotificationArchiveShard(item.user)
		_, err := tx.ExecContext(ctx, SQLQueriesPackage.HGNotificationLedgerInsertSQL, shard, item.eventID, item.user, item.category)
		if err != nil && !hgNotificationDuplicate(err) {
			return after, 0, err
		}
		if _, err := tx.ExecContext(ctx, SQLQueriesPackage.HGNotificationArchiveInsertSQL, shard, item.id, item.eventID, item.user, item.category, item.title, item.content, item.targetType, item.targetID, item.read, item.deleted, item.created); err != nil {
			return after, 0, err
		}
		result, err := tx.ExecContext(ctx, SQLQueriesPackage.HGNotificationArchiveDeleteSQL, item.id)
		if err != nil {
			return after, 0, err
		}
		affected, err := result.RowsAffected()
		if err != nil || affected != 1 {
			return after, 0, fmt.Errorf("归档删除行数异常，事务回滚: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return after, 0, err
	}
	if len(items) == 100 {
		return items[len(items)-1].id, len(items), nil
	}
	return upper, len(items), nil
}
