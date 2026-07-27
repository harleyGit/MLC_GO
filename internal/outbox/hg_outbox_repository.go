package outbox

import (
	"MLC_GO/internal/events"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const defaultTopic = "mlc.domain.events"

// Repository 封装 Outbox 表访问。
// Service 只负责把领域事件交给 event bus；Repository 负责把事件可靠落到 MySQL 本地消息表。
type Repository struct {
	db    *sql.DB
	topic string
}

// Event 是 dispatcher 从 outbox_events 拉出来的一条待投递消息。
type Event struct {
	// ID 是 outbox_events 表自增主键，仅用于本地状态更新。
	ID int64
	// EventID 是跨服务事件唯一 ID，用于消费侧幂等和排障。
	EventID string
	// EventName 是业务事件名称，例如 video.published。
	EventName string
	// EventKey 是分区路由 key，通常为 submission_id/order_id/user_id。
	EventKey string
	// Topic 是目标 Kafka topic，允许未来按事件域拆分 topic。
	Topic string
	// Payload 是已经序列化的 EventEnvelope 字节，dispatcher 不再二次 marshal。
	Payload []byte
	// RetryCount 是已失败次数，用于判断是否进入 dead 状态。
	RetryCount int
	// LeaseToken 是当前 dispatcher claim 的 fencing token，过期 worker 无权 ack 新租约。
	LeaseToken string
}

// NewRepository 创建 Outbox 仓储。
func NewRepository(db *sql.DB, topic string) *Repository {
	if topic == "" {
		// 未显式配置时使用项目统一领域事件 topic，避免写入空 topic 导致 dispatcher 无法投递。
		topic = defaultTopic
	}
	return &Repository{db: db, topic: topic}
}

// Save 把领域事件写入 Outbox。
// 初学者重点：这里不是直接发 Kafka，而是先写 MySQL；后续 dispatcher 再异步投递 Kafka。
func (r *Repository) Save(ctx context.Context, event events.DomainEvent) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("outbox repository db cannot be nil")
	}
	// Envelope 固化跨服务协议字段，避免直接把业务结构裸写入 Kafka。
	envelope, err := events.NewEnvelope(event)
	if err != nil {
		return err
	}
	// Outbox 表保存最终要投递到 Kafka 的字节，确保失败重试时内容稳定不变。
	payload, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal outbox envelope: %w", err)
	}
	return r.SaveEnvelopeExec(ctx, r.db, envelope, payload)
}

// SaveTx 在业务事务内写入 Outbox。
// 这是 Outbox Pattern 的关键点：业务数据和事件记录共用同一个 commit。
func (r *Repository) SaveTx(ctx context.Context, tx *sql.Tx, event events.DomainEvent) error {
	// 由调用方传入业务事务 tx，保证业务数据和 outbox 事件一起提交或一起回滚。
	envelope, err := events.NewEnvelope(event)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal outbox envelope: %w", err)
	}
	return r.SaveEnvelopeExec(ctx, tx, envelope, payload)
}

type sqlExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// SaveEnvelopeExec 使用传入的执行器写入 Outbox，兼容 *sql.DB 和 *sql.Tx。
func (r *Repository) SaveEnvelopeExec(ctx context.Context, execer sqlExecer, envelope events.EventEnvelope, payload []byte) error {
	// 数据库 event_id 与 Kafka envelope.EventID 必须一致，消费侧才能按同一个 ID 原子去重。
	_, err := execer.ExecContext(ctx, SQLQueriesPackage.InsertOutboxEventSQL, envelope.EventID, envelope.EventName, envelope.EventKey, r.topic, payload)
	return err
}

// FetchPendingTx 在事务内锁定并读取一批待投递事件。
// FOR UPDATE SKIP LOCKED 可以让多个 dispatcher 并发抢任务时不重复处理同一行。
func (r *Repository) FetchPendingTx(ctx context.Context, tx *sql.Tx, limit int) ([]Event, error) {
	if limit <= 0 || limit > 1000 {
		// 限制单批大小，避免一次锁定过多行影响业务写入和其他 dispatcher。
		limit = 100
	}
	rows, err := tx.QueryContext(ctx, SQLQueriesPackage.SelectPendingOutboxEventsSQL, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]Event, 0, limit)
	for rows.Next() {
		var event Event
		// Scan 字段顺序必须与 SelectPendingOutboxEventsSQL 保持一致。
		if err := rows.Scan(&event.ID, &event.EventID, &event.EventName, &event.EventKey, &event.Topic, &event.Payload, &event.RetryCount); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

// Claim 在短事务内领取一批事件并写入租约，事务提交后才返回，调用方可在事务外发送 Kafka。
func (r *Repository) Claim(ctx context.Context, limit int, leaseDuration time.Duration) ([]Event, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("outbox repository db cannot be nil")
	}
	if leaseDuration <= 0 {
		leaseDuration = 30 * time.Second
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("begin outbox claim: %w", err)
	}
	defer tx.Rollback()
	events, err := r.FetchPendingTx(ctx, tx, limit)
	if err != nil {
		return nil, fmt.Errorf("select outbox claim: %w", err)
	}
	leaseSeconds := int64(leaseDuration / time.Second)
	if leaseSeconds < 1 {
		leaseSeconds = 1
	}
	for i := range events {
		events[i].LeaseToken = uuid.NewString()
		result, execErr := tx.ExecContext(ctx, SQLQueriesPackage.ClaimOutboxEventSQL, events[i].LeaseToken, leaseSeconds, events[i].ID)
		if execErr != nil {
			return nil, fmt.Errorf("lease outbox event %d: %w", events[i].ID, execErr)
		}
		if affected, affectedErr := result.RowsAffected(); affectedErr != nil || affected != 1 {
			return nil, fmt.Errorf("lease outbox event %d affected %d rows: %w", events[i].ID, affected, affectedErr)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit outbox claim: %w", err)
	}
	return events, nil
}

// MarkPublished 使用 fencing token 确认 Kafka 已成功接收消息。
func (r *Repository) MarkPublished(ctx context.Context, id int64, leaseToken string) (bool, error) {
	result, err := r.db.ExecContext(ctx, SQLQueriesPackage.MarkOutboxEventPublishedSQL, id, leaseToken)
	return hgOutboxUpdateResult(result, err)
}

// MarkRetry 释放租约并安排指数退避后的下一次投递。
func (r *Repository) MarkRetry(ctx context.Context, id int64, leaseToken string, reason string, delay time.Duration) (bool, error) {
	result, err := r.db.ExecContext(ctx, SQLQueriesPackage.MarkOutboxEventRetrySQL, hgTruncateOutboxError(reason), int64(delay/time.Second), id, leaseToken)
	return hgOutboxUpdateResult(result, err)
}

// MarkDead 使用 fencing token 将超过上限的消息移入 dead 状态。
func (r *Repository) MarkDead(ctx context.Context, id int64, leaseToken string, reason string) (bool, error) {
	result, err := r.db.ExecContext(ctx, SQLQueriesPackage.MarkOutboxEventDeadSQL, hgTruncateOutboxError(reason), id, leaseToken)
	return hgOutboxUpdateResult(result, err)
}

func hgOutboxUpdateResult(result sql.Result, err error) (bool, error) {
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected == 1, err
}

func hgTruncateOutboxError(reason string) string {
	const maxRunes = 1000
	if utf8.RuneCountInString(reason) <= maxRunes {
		return reason
	}
	runes := []rune(reason)
	return string(runes[:maxRunes])
}
