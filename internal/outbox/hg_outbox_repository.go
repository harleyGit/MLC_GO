package outbox

import (
	"MLC_GO/internal/events"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

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
	// event_id 每次落库生成，业务幂等 key 仍使用 envelope.EventKey 传递给 Kafka。
	_, err := execer.ExecContext(ctx, SQLQueriesPackage.InsertOutboxEventSQL, uuid.NewString(), envelope.EventName, envelope.EventKey, r.topic, payload)
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

// MarkPublishedTx 将事件标记为已发布。
func (r *Repository) MarkPublishedTx(ctx context.Context, tx *sql.Tx, id int64) error {
	_, err := tx.ExecContext(ctx, SQLQueriesPackage.MarkOutboxEventPublishedSQL, id)
	return err
}

// MarkRetryTx 记录一次投递失败并保留 pending 状态，等待下一轮重试。
func (r *Repository) MarkRetryTx(ctx context.Context, tx *sql.Tx, id int64, reason string) error {
	_, err := tx.ExecContext(ctx, SQLQueriesPackage.MarkOutboxEventRetrySQL, reason, id)
	return err
}

// MarkDeadTx 将超过重试上限的事件标记为 dead，避免无限重试拖垮队列。
func (r *Repository) MarkDeadTx(ctx context.Context, tx *sql.Tx, id int64, reason string) error {
	_, err := tx.ExecContext(ctx, SQLQueriesPackage.MarkOutboxEventDeadSQL, reason, id)
	return err
}

// dbConn 返回底层数据库连接，仅供 dispatcher 统一开启事务。
func (r *Repository) dbConn() *sql.DB { return r.db }
