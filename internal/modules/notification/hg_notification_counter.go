package notification

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"hash/crc32"
	"sort"

	"github.com/go-sql-driver/mysql"
)

const hgNotificationCountShards = 16

func hgNotificationArchiveShard(user string) byte { return byte(crc32.ChecksumIEEE([]byte(user)) % 16) }

func hgNotificationDuplicate(err error) bool {
	var typed *mysql.MySQLError
	return errors.As(err, &typed) && typed.Number == 1062
}

// HGIgnore 记录当前偏好拒收的业务事件，防止用户日后开启分类时 Kafka 重放补发历史通知。
func (repo *HGRepository) HGIgnore(ctx context.Context, eventID, user, category string) error {
	_, err := repo.db.ExecContext(ctx, SQLQueriesPackage.HGNotificationLedgerInsertSQL, hgNotificationArchiveShard(user), eventID, user, category)
	if hgNotificationDuplicate(err) {
		return nil
	}
	return err
}

func hgNotificationCounterReady(ctx context.Context, tx *sql.Tx) (bool, error) {
	var ready bool
	err := tx.QueryRowContext(ctx, SQLQueriesPackage.HGNotificationCounterReadySQL).Scan(&ready)
	return ready, err
}

// hgDeliverCount 在单事务内登记永久事件墓碑、写入收件箱并增加计数；重复事件不再次入箱。
func (repo *HGRepository) hgDeliverCount(ctx context.Context, eventID, user string, notice HGNotice) error {
	tx, err := repo.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, SQLQueriesPackage.HGNotificationLedgerInsertSQL, hgNotificationArchiveShard(user), eventID, user, notice.Category)
	if hgNotificationDuplicate(err) {
		return nil
	}
	if err != nil {
		return err
	}
	ready, err := hgNotificationCounterReady(ctx, tx)
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, SQLQueriesPackage.HGNotificationInsertSQL, eventID, user, notice.Category, notice.Title, notice.Content, notice.TargetType, notice.TargetID)
	if hgNotificationDuplicate(err) {
		// 升级前老通知已存在但 ledger 尚未回填；保留新登记的去重键，不重复增加计数。
		return tx.Commit()
	}
	if err != nil {
		return err
	}
	if ready {
		id, idErr := result.LastInsertId()
		if idErr != nil || id <= 0 {
			return fmt.Errorf("无法取得通知主键: %w", idErr)
		}
		if _, err := tx.ExecContext(ctx, SQLQueriesPackage.HGNotificationCounterIncreaseSQL, user, notice.Category, id%hgNotificationCountShards); err != nil {
			return err
		}
	}
	return tx.Commit()
}

type hgNotificationCountKey struct {
	category string
	shard    byte
}

func hgNotificationApplyDecrease(ctx context.Context, tx *sql.Tx, user string, keys map[hgNotificationCountKey]int64) error {
	ordered := make([]hgNotificationCountKey, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].category == ordered[j].category {
			return ordered[i].shard < ordered[j].shard
		}
		return ordered[i].category < ordered[j].category
	})
	for _, key := range ordered {
		n := keys[key]
		result, err := tx.ExecContext(ctx, SQLQueriesPackage.HGNotificationCounterDecreaseSQL, n, user, key.category, key.shard, n)
		if err != nil {
			return err
		}
		rows, err := result.RowsAffected()
		if err != nil || rows != 1 {
			return fmt.Errorf("通知计数不一致，已回滚: %w", err)
		}
	}
	return nil
}

func hgNotificationIDArgs(user string, ids []uint64) []any {
	args := make([]any, 0, 1+len(ids))
	args = append(args, user)
	for _, id := range ids {
		args = append(args, id)
	}
	return args
}

func (repo *HGRepository) hgReadIDsCount(ctx context.Context, user string, ids []uint64) error {
	query := SQLQueriesPackage.HGNotificationSelectUnreadIDsSQL(len(ids))
	if query == "" {
		return HGErrInvalid
	}
	tx, err := repo.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, query, hgNotificationIDArgs(user, ids)...)
	if err != nil {
		return err
	}
	selected := make([]uint64, 0, len(ids))
	counts := make(map[hgNotificationCountKey]int64)
	for rows.Next() {
		var id uint64
		var category string
		if err = rows.Scan(&id, &category); err != nil {
			break
		}
		selected = append(selected, id)
		counts[hgNotificationCountKey{category, byte(id % hgNotificationCountShards)}]++
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return err
	}
	if len(selected) == 0 {
		return tx.Commit()
	}
	ready, err := hgNotificationCounterReady(ctx, tx)
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, SQLQueriesPackage.HGNotificationSetReadSelectedSQL(len(selected)), hgNotificationIDArgs(user, selected)...)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != int64(len(selected)) {
		return fmt.Errorf("通知已读写入不一致，已回滚: %w", err)
	}
	if ready {
		if err := hgNotificationApplyDecrease(ctx, tx, user, counts); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (repo *HGRepository) hgReadBatchCount(ctx context.Context, user, category string) (bool, error) {
	if !hgCategories[category] {
		return false, HGErrInvalid
	}
	tx, err := repo.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, SQLQueriesPackage.HGNotificationSelectUnreadBatchSQL, user, category)
	if err != nil {
		return false, err
	}
	ids := make([]uint64, 0, 500)
	counts := make(map[hgNotificationCountKey]int64)
	for rows.Next() {
		var id uint64
		if err = rows.Scan(&id); err != nil {
			break
		}
		ids = append(ids, id)
		counts[hgNotificationCountKey{category, byte(id % hgNotificationCountShards)}]++
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return false, err
	}
	if len(ids) == 0 {
		return false, tx.Commit()
	}
	ready, err := hgNotificationCounterReady(ctx, tx)
	if err != nil {
		return false, err
	}
	result, err := tx.ExecContext(ctx, SQLQueriesPackage.HGNotificationSetReadSelectedSQL(len(ids)), hgNotificationIDArgs(user, ids)...)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != int64(len(ids)) {
		return false, fmt.Errorf("通知批量已读不一致，已回滚: %w", err)
	}
	if ready {
		if err := hgNotificationApplyDecrease(ctx, tx, user, counts); err != nil {
			return false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return len(ids) == 500, nil
}

func (repo *HGRepository) hgDeleteCount(ctx context.Context, user string, id uint64) error {
	tx, err := repo.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var category string
	var read bool
	if err := tx.QueryRowContext(ctx, SQLQueriesPackage.HGNotificationSelectDeleteSQL, user, id).Scan(&category, &read); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	ready, err := hgNotificationCounterReady(ctx, tx)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, SQLQueriesPackage.HGNotificationDeleteSQL, user, id); err != nil {
		return err
	}
	if ready && !read {
		if err := hgNotificationApplyDecrease(ctx, tx, user, map[hgNotificationCountKey]int64{{category, byte(id % hgNotificationCountShards)}: 1}); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// HGRebuildUnreadOffline 必须先停止此用户的所有通知写入/已读/删除，按用户原始未读索引重建分片；不提供 HTTP 入口。
// 热用户可能有海量未读，此方法只适合隔离窗口且应先评估索引扫描成本；完成所有用户后再切换全局 ready。
func (repo *HGRepository) HGRebuildUnreadOffline(ctx context.Context, user string) error {
	if user == "" || len(user) > 128 {
		return HGErrInvalid
	}
	counts := make(map[hgNotificationCountKey]int64)
	var cursor uint64
	seen := 0
	for {
		rows, err := repo.db.QueryContext(ctx, SQLQueriesPackage.HGNotificationRebuildPageSQL, user, cursor)
		if err != nil {
			return err
		}
		n := 0
		for rows.Next() {
			var id uint64
			var category string
			if err = rows.Scan(&id, &category); err != nil {
				break
			}
			cursor = id
			counts[hgNotificationCountKey{category, byte(id % hgNotificationCountShards)}]++
			n++
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
		if err != nil {
			return err
		}
		seen += n
		if seen > 10000 {
			return fmt.Errorf("用户未读超过离线单用户重建上限，须另行分批预聚合")
		}
		if n < 501 {
			break
		}
	}
	tx, err := repo.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, SQLQueriesPackage.HGNotificationDeleteUserShardsSQL, user); err != nil {
		return err
	}
	ordered := make([]hgNotificationCountKey, 0, len(counts))
	for key := range counts {
		ordered = append(ordered, key)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].category == ordered[j].category {
			return ordered[i].shard < ordered[j].shard
		}
		return ordered[i].category < ordered[j].category
	})
	for _, key := range ordered {
		if _, err := tx.ExecContext(ctx, SQLQueriesPackage.HGNotificationSetUserShardSQL, user, key.category, key.shard, counts[key]); err != nil {
			return err
		}
	}
	return tx.Commit()
}
