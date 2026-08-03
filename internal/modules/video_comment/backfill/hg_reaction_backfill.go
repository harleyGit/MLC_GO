package VideoCommentBackfillPackage

import (
	"context"
	"database/sql"
	"fmt"
)

const hgReactionBackfillJob = "reaction_shards_v1"

const (
	// checkpoint 行是回填单写者锁和恢复游标。FOR UPDATE 使多个误启动的命令串行，条件推进防止陈旧执行者覆盖新游标。
	hgEnsureCheckpointSQL = `INSERT INTO video_comment_reaction_backfill_state (job_name, cursor_id, completed) VALUES (?, 0, 0) ON DUPLICATE KEY UPDATE job_name = VALUES(job_name)`
	hgSelectCheckpointSQL = `SELECT cursor_id, completed FROM video_comment_reaction_backfill_state WHERE job_name = ? FOR UPDATE`
	// 初始游标为 0 时 shard 表必须为空，否则无法区分旧版 000022 已回填数据与部分脏数据，继续累加会导致翻倍。
	hgCountShardsSQL = `SELECT COUNT(*) FROM video_comment_reaction_shards`
	// 先用主键 keyset 找到本批末端，再在闭区间内聚合；不使用 OFFSET，批次成本不会随游标推进而增长。
	hgSelectBatchEndSQL = `SELECT COALESCE(MAX(id), ?) FROM (SELECT id FROM video_comment_reactions WHERE id > ? ORDER BY id ASC LIMIT ?) bounded_reactions`
	hgAggregateBatchSQL = `INSERT INTO video_comment_reaction_shards (comment_id, shard_id, like_count, dislike_count)
SELECT comment_id, CRC32(user_id) % 32, SUM(reaction = 'like'), SUM(reaction = 'dislike')
FROM video_comment_reactions
WHERE id > ? AND id <= ?
GROUP BY comment_id, CRC32(user_id) % 32
ON DUPLICATE KEY UPDATE like_count = like_count + VALUES(like_count), dislike_count = dislike_count + VALUES(dislike_count)`
	hgAdvanceCheckpointSQL = `UPDATE video_comment_reaction_backfill_state SET cursor_id = ?, completed = ? WHERE job_name = ? AND cursor_id = ?`
)

// HGReactionBackfill 按 video_comment_reactions.id 自增主键执行可暂停、可恢复的有界分片回填。
// 每批只扫描 (cursor_id, batch_end]，并在同一事务中累加 shard 和推进 checkpoint；事务失败时两者一起回滚，重启不会重复已提交批次。
// 回填期间 reaction API 通过同一 checkpoint 的 completed=false 状态 fail closed，避免在线 delta 双写与历史绝对关系聚合重复计数。
type HGReactionBackfill struct{ db *sql.DB }

// NewHGReactionBackfill 创建分片回填器。
func NewHGReactionBackfill(db *sql.DB) *HGReactionBackfill { return &HGReactionBackfill{db: db} }

// RunBatch 将 checkpoint 和一个主键区间的增量聚合放在同一 READ COMMITTED 短事务中。
// processed 表示本轮提交了一个非空区间，completed 表示已无后续关系行；调用方应在批次间限速并为每轮设置独立超时。
func (b *HGReactionBackfill) RunBatch(ctx context.Context, batchSize int) (processed bool, completed bool, err error) {
	if b == nil || b.db == nil || batchSize < 1 || batchSize > 100000 {
		return false, false, fmt.Errorf("reaction backfill batch size must be between 1 and 100000")
	}
	tx, err := b.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return false, false, fmt.Errorf("begin reaction backfill: %w", err)
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, hgEnsureCheckpointSQL, hgReactionBackfillJob); err != nil {
		return false, false, fmt.Errorf("ensure reaction backfill checkpoint: %w", err)
	}
	var cursor uint64
	if err = tx.QueryRowContext(ctx, hgSelectCheckpointSQL, hgReactionBackfillJob).Scan(&cursor, &completed); err != nil {
		return false, false, fmt.Errorf("read reaction backfill checkpoint: %w", err)
	}
	if completed {
		// 已完成状态是幂等终点，重复运行命令只读取并提交，不再触碰关系表或 shard 表。
		return false, true, tx.Commit()
	}
	if cursor == 0 {
		var shardCount uint64
		if err = tx.QueryRowContext(ctx, hgCountShardsSQL).Scan(&shardCount); err != nil {
			return false, false, fmt.Errorf("count existing reaction shards: %w", err)
		}
		if shardCount > 0 {
			return false, false, fmt.Errorf("reaction shard table is not empty before initial backfill")
		}
	}
	var batchEnd uint64
	if err = tx.QueryRowContext(ctx, hgSelectBatchEndSQL, cursor, cursor, batchSize).Scan(&batchEnd); err != nil {
		return false, false, fmt.Errorf("read reaction backfill batch end: %w", err)
	}
	if batchEnd == cursor {
		// 没有更大主键时在事务内标记完成；reaction API 随后才能恢复权威分片写入。
		result, updateErr := tx.ExecContext(ctx, hgAdvanceCheckpointSQL, cursor, true, hgReactionBackfillJob, cursor)
		if updateErr != nil {
			return false, false, fmt.Errorf("complete reaction backfill: %w", updateErr)
		}
		if affected, affectedErr := result.RowsAffected(); affectedErr != nil || affected != 1 {
			return false, false, fmt.Errorf("complete reaction backfill checkpoint conflict")
		}
		return false, true, tx.Commit()
	}
	if _, err = tx.ExecContext(ctx, hgAggregateBatchSQL, cursor, batchEnd); err != nil {
		return false, false, fmt.Errorf("aggregate reaction backfill batch: %w", err)
	}
	result, err := tx.ExecContext(ctx, hgAdvanceCheckpointSQL, batchEnd, false, hgReactionBackfillJob, cursor)
	if err != nil {
		return false, false, fmt.Errorf("advance reaction backfill checkpoint: %w", err)
	}
	if affected, affectedErr := result.RowsAffected(); affectedErr != nil || affected != 1 {
		return false, false, fmt.Errorf("advance reaction backfill checkpoint conflict")
	}
	if err = tx.Commit(); err != nil {
		return false, false, fmt.Errorf("commit reaction backfill: %w", err)
	}
	return true, false, nil
}
