package send

import (
	notificationevents "MLC_GO/internal/events/notification"
	queries "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

func (worker *HGWorker) hgClaimReady(ctx context.Context, event notificationevents.HGSendReadyEvent) (*hgJob, error) {
	id, err := strconv.ParseUint(event.TaskID, 10, 64)
	if err != nil {
		return nil, HGErrInvalid
	}
	tx, err := worker.repo.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	job := &hgJob{id: id, mode: "kafka", generation: event.Generation}
	var audience, variables []byte
	err = tx.QueryRowContext(ctx, queries.HGNotificationKafkaClaimSQL, id, event.Generation, event.NextIndex).Scan(
		&job.template, &job.version, &audience, &variables, &job.index, &job.attempts)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil // 旧代次、取消或已处理事件；不得重复入箱。
	}
	if err != nil {
		return nil, err
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	job.token = hex.EncodeToString(nonce[:])
	result, err := tx.ExecContext(ctx, queries.HGNotificationKafkaLeaseSQL,
		job.token, id, event.Generation, event.NextIndex)
	if err != nil {
		return nil, err
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return nil, fmt.Errorf("通知 Kafka 租约未生效: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	if json.Unmarshal(audience, &job.users) != nil || json.Unmarshal(variables, &job.vars) != nil ||
		len(job.users) == 0 || len(job.users) > 100 || job.index < 0 || job.index >= len(job.users) {
		// 快照损坏不可把未知用户写入站内信；仅当前租约有权标为失败。
		result, err := worker.repo.db.ExecContext(ctx, queries.HGNotificationKafkaClaimCorruptSQL, id, job.token, event.Generation)
		if err != nil {
			return nil, err
		}
		if n, err := result.RowsAffected(); err != nil || n != 1 {
			return nil, fmt.Errorf("通知 Kafka 损坏任务未隔离: %w", err)
		}
		return nil, nil
	}
	return job, nil
}
