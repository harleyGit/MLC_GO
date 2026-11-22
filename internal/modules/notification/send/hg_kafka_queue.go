package send

import (
	notificationevents "MLC_GO/internal/events/notification"
	"MLC_GO/internal/outbox"
	queries "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"time"
)

// HGQueueKafkaOnce 将一个到期 Kafka 模式任务的当前游标与下一代次在同一 MySQL 事务写入 Outbox。
// 多 Pod 通过 SKIP LOCKED 抢占；超时的 queued/leased 任务产生新代次，旧 Kafka 事件不可重领。
func (repo *HGRepository) HGQueueKafkaOnce(ctx context.Context) (bool, error) {
	return repo.hgQueueKafkaOnce(ctx, int(time.Now().Unix()%3))
}

func (repo *HGRepository) hgQueueKafkaOnce(ctx context.Context, first int) (bool, error) {
	if repo == nil || repo.db == nil {
		return false, fmt.Errorf("通知 Kafka 调度依赖未就绪")
	}
	tx, err := repo.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	queriesByStatus := []string{queries.HGNotificationKafkaReadySQL,
		queries.HGNotificationKafkaQueuedExpiredSQL, queries.HGNotificationKafkaLeasedExpiredSQL}
	// 新任务和超时事件轮流获得优先级；一次只扫描三个有界索引前缀。
	var id, generation uint64
	var nextIndex, recipients int
	for n := range queriesByStatus {
		query := queriesByStatus[(first+n)%len(queriesByStatus)]
		err = tx.QueryRowContext(ctx, query).Scan(&id, &nextIndex, &generation, &recipients)
		if !errors.Is(err, sql.ErrNoRows) {
			break
		}
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if id == 0 || id > math.MaxInt64 || recipients < 1 || recipients > 100 ||
		nextIndex < 0 || nextIndex >= recipients || generation == math.MaxUint64 {
		if _, err := tx.ExecContext(ctx, queries.HGNotificationKafkaCorruptSQL, id); err != nil {
			return false, err
		}
		if err := tx.Commit(); err != nil {
			return false, err
		}
		return false, HGErrInvalid
	}
	event, err := notificationevents.HGNewSendReadyEvent(ctx, id, uint16(nextIndex), generation+1)
	if err != nil {
		return false, err
	}
	result, err := tx.ExecContext(ctx, queries.HGNotificationKafkaQueueSQL, generation+1, id, generation, nextIndex)
	if err != nil {
		return false, err
	}
	if changed, checkErr := result.RowsAffected(); checkErr != nil || changed != 1 {
		return false, fmt.Errorf("通知 Kafka 代次更新失败: %w", checkErr)
	}
	// 使用现有领域事件 Envelope 与 dispatcher；不能在提交前直接发布 Kafka。
	outboxID, err := outbox.NewRepository(nil, notificationevents.HGSendReadyTopic).SaveTxID(ctx, tx, event)
	if err != nil {
		return false, err
	}
	if outboxID <= 0 {
		return false, fmt.Errorf("通知 Kafka 出站主键无效")
	}
	result, err = tx.ExecContext(ctx, queries.HGNotificationKafkaBindOutboxSQL, outboxID, id, generation+1)
	if err != nil {
		return false, err
	}
	if changed, checkErr := result.RowsAffected(); checkErr != nil || changed != 1 {
		return false, fmt.Errorf("通知 Kafka 出站绑定失败: %w", checkErr)
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}
