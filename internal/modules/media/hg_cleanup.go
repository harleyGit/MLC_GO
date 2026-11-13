package media

import (
	SQL "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

type hgDeleteJob struct {
	id       uint64 // 删除任务主键，用于 CAS 与故障恢复。
	asset    string // 归属资源 ID，必须已进入 deleting。
	bucket   string // 存储桶快照，必须与受信配置相同。
	key      string // 存储对象键，只允许隔离与封存前缀。
	attempts int    // 本轮领取前的失败/超时次数。
	lease    []byte // 16 字节随机租约，拒绝旧 worker 回写。
}

// HGExpireOnce 每次最多处理一个超过 24 小时保护期的上传；有引用则转 attached，不释放对象。
func (s *HGService) HGExpireOnce(ctx context.Context) error {
	if s.sealed == nil {
		return errors.New("媒体删除存储未配置")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var upload, asset, key, status string
	queries := [...]string{SQL.HGMediaExpireClaimSQL, SQL.HGMediaExpireProcessingSQL, SQL.HGMediaExpireFailedSQL, SQL.HGMediaExpireReadySQL}
	start := int(s.expireCursor.Add(1)-1) % len(queries)
	for i := 0; i < len(queries); i++ {
		err = tx.QueryRowContext(ctx, queries[(start+i)%len(queries)]).Scan(&upload, &asset, &key, &status)
		if !errors.Is(err, sql.ErrNoRows) {
			break
		}
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var assetStatus string
	var sealed sql.NullString
	if err = tx.QueryRowContext(ctx, SQL.HGMediaExpireAssetLockSQL, asset).Scan(&assetStatus, &sealed); err != nil {
		return err
	}
	if assetStatus != status || !hgQuarantineKey.MatchString(key) || key != "quarantine/"+strings.ReplaceAll(asset, "-", "") || (sealed.Valid && !hgSealedKey.MatchString(sealed.String)) {
		return HGErrConflict
	}
	var found int
	err = tx.QueryRowContext(ctx, SQL.HGMediaHasRefSQL, asset).Scan(&found)
	if err == nil {
		if status != "ready" {
			return HGErrConflict
		}
		_, err = tx.ExecContext(ctx, SQL.HGMediaExpireAttachSQL, upload)
		if err != nil {
			return err
		}
		return tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	res, err := tx.ExecContext(ctx, SQL.HGMediaExpireAssetSQL, asset)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil || n != 1 {
		return HGErrConflict
	}
	res, err = tx.ExecContext(ctx, SQL.HGMediaExpireSessionSQL, upload)
	if err != nil {
		return err
	}
	n, err = res.RowsAffected()
	if err != nil || n != 1 {
		return HGErrConflict
	}
	if _, err = tx.ExecContext(ctx, SQL.HGMediaDeleteQueueSQL, asset, s.bucket, key); err != nil {
		return err
	}
	// 即使审核器响应丢失，也必须清理其幂等封存键，避免产生无元数据孤儿对象。
	sealedKey := "sealed/" + strings.ReplaceAll(asset, "-", "")
	if sealed.Valid && sealed.String != sealedKey {
		return HGErrConflict
	}
	if _, err = tx.ExecContext(ctx, SQL.HGMediaDeleteQueueSQL, asset, s.bucket, sealedKey); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *HGService) hgClaimDelete(ctx context.Context) (hgDeleteJob, error) {
	var job hgDeleteJob
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return job, err
	}
	defer tx.Rollback()
	queries := [...]string{SQL.HGMediaDeleteClaimSQL, SQL.HGMediaDeleteReclaimSQL}
	start := int(s.deleteCursor.Add(1)-1) % len(queries)
	for i := 0; i < len(queries); i++ {
		err = tx.QueryRowContext(ctx, queries[(start+i)%len(queries)]).Scan(&job.id, &job.asset, &job.bucket, &job.key, &job.attempts)
		if !errors.Is(err, sql.ErrNoRows) {
			break
		}
	}
	if err != nil {
		return job, err
	}
	job.lease = make([]byte, 16)
	if _, err = rand.Read(job.lease); err != nil {
		return job, err
	}
	res, err := tx.ExecContext(ctx, SQL.HGMediaDeleteLeaseSQL, job.lease, job.id)
	if err != nil {
		return job, err
	}
	n, err := res.RowsAffected()
	if err != nil || n != 1 {
		return job, HGErrConflict
	}
	return job, tx.Commit()
}

// HGDeleteOnce 从持久任务表领取一件，只有私有隔离桶的服务端生成 Key 能被物理删除。
func (s *HGService) HGDeleteOnce(ctx context.Context) error {
	if s.sealed == nil {
		return errors.New("媒体删除存储未配置")
	}
	job, err := s.hgClaimDelete(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("领取媒体删除任务失败: %w", err)
	}
	status := "done"
	var deleteErr error
	validKey := hgQuarantineKey.MatchString(job.key) && job.key == "quarantine/"+strings.ReplaceAll(job.asset, "-", "") || hgSealedKey.MatchString(job.key) && job.key == "sealed/"+strings.ReplaceAll(job.asset, "-", "")
	if job.attempts >= 12 || job.bucket != s.bucket || !validKey {
		status = "failed"
	} else {
		deleteErr = s.sealed.HGDelete(ctx, job.key)
		if deleteErr != nil {
			status = "pending"
			if job.attempts+1 >= 12 {
				status = "failed"
			}
		}
	}
	if err = s.hgFinishDelete(ctx, job, status); err != nil {
		return fmt.Errorf("保存媒体删除结果失败: %w", err)
	}
	return deleteErr
}

func (s *HGService) hgFinishDelete(ctx context.Context, job hgDeleteJob, status string) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// CAS token 防止租约过期后的旧 worker 覆盖新领取者；update 原子检查即可。
	backoff := min(30*(job.attempts+1), 3600)
	res, err := tx.ExecContext(ctx, SQL.HGMediaDeleteFinishSQL, status, backoff, job.id, job.lease)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil || n != 1 {
		return HGErrConflict
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	if status != "done" {
		return nil
	}
	// 不在持有 job 锁时获取 asset 锁，避免与过期任务的 asset -> job 顺序形成环。
	return s.hgFinalizeDeleted(ctx, job.asset)
}

func (s *HGService) hgFinalizeDeleted(ctx context.Context, asset string) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var state string
	var sealed sql.NullString
	if err = tx.QueryRowContext(ctx, SQL.HGMediaExpireAssetLockSQL, asset).Scan(&state, &sealed); err != nil {
		return err
	}
	if state == "deleted" {
		if _, err = tx.ExecContext(ctx, SQL.HGMediaMarkFinalizedSQL, asset); err != nil {
			return err
		}
		return tx.Commit()
	}
	if state != "deleting" {
		return HGErrConflict
	}
	var remaining int
	err = tx.QueryRowContext(ctx, SQL.HGMediaDeleteRemainingSQL, asset).Scan(&remaining)
	if err == nil {
		if _, err = tx.ExecContext(ctx, SQL.HGMediaMarkFinalizedSQL, asset); err != nil {
			return err
		}
		return tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if _, err = tx.ExecContext(ctx, SQL.HGMediaDeleteFinalAssetSQL, asset); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, SQL.HGMediaDeleteFinalSessionSQL, asset); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, SQL.HGMediaMarkFinalizedSQL, asset); err != nil {
		return err
	}
	return tx.Commit()
}

// hgRecoverFinalized 修复“最后一个 S3 删除成功、DB 标记完成后进程退出”的一致性窗口。
func (s *HGService) hgRecoverFinalized(ctx context.Context) error {
	var asset string
	err := s.db.QueryRowContext(ctx, SQL.HGMediaDeleteRecoverSQL).Scan(&asset)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.hgFinalizeDeleted(ctx, asset)
}
