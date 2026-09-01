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
// 数据库事务实现的 Outbox 模式（发件箱模式）抢占认领逻辑，典型用于可靠消息投递：把待发送事件从数据库捞出来，**加锁租约 (lease)**，交给业务去发送；别的 worker 不能重复认领同一条事件。
// 整体流程：参数校验 → 开启事务 → 查询待处理事件 → 循环逐个更新租约令牌 + 租约过期时间 → 校验每一行更新生效 → 提交事务返回认领成功的事件
//
//	@param ctx 上下文，传递超时、取消信号，数据库操作全程复用
//	@param limit 最多认领多少条待处理事件，防止一次性拉取太多压垮服务
//	@param leaseDuration 租约时长，worker 拿到事件之后，在这个时间内拥有这条事件的独占处理权；租约到期后其他 worker 可以再次抢占这条事件。
//	@return []Event 认领成功的事件列表，上层拿到后执行消息发送
//	@return error 任意一步出错返回错误，事务会回滚，不会认领任何数据。
func (r *Repository) Claim(ctx context.Context, limit int, leaseDuration time.Duration) ([]Event, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("outbox repository db cannot be nil")
	}
	if leaseDuration <= 0 {
		// 租约太短：worker 还没处理完就过期，会出现多 worker 重复处理；租约太长：worker 宕机后，这条消息要等很久才会被重新消费。
		leaseDuration = 30 * time.Second
	}
	
	// 开启数据库事务，隔离级别为 Read Committed，保证读取到的行是已经提交的最新数据。
	// 带 context 开启事务，context 取消的时候事务会被中断。
	// 隔离级别：ReadCommitted 读已提交。Outbox 认领场景够用，不需要更高隔离级别（RepeatableRead / Serializable 会带来锁竞争、死锁）
	// 是在配置：这个事务使用什么隔离级别； sql.LevelReadCommitted 读已提交
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("begin outbox claim: %w", err)
	}
	// 只要没有走到 `tx.Commit()`，函数退出时一定执行 Rollback
	// Commit 成功之后，Rollback 调用是无害的，数据库驱动会忽略已经提交的事务的回滚。
	defer tx.Rollback()
	// FetchPendingTx 是内部方法，传入事务 tx【FetchPendingTx 里面执行的 SQL 属于当前事务。】，在当前事务里面查询状态为 pending、未被租约占用的 outbox 事件，最多 limit 条
	// 重点：是在事务 tx 之上做查询，不是 r.db 裸库查询。
	// 这里 SQL 一般会做行锁（`SELECT ... FOR UPDATE`），锁住这些待处理行，防止其他并发 Claim 同时读到并抢占同一批数据
	events, err := r.FetchPendingTx(ctx, tx, limit)
	if err != nil {
		return nil, fmt.Errorf("select outbox claim: %w", err)
	}

	// leaseSeconds 把 time.Duration 转为秒；极端情况保证最少 1 秒租约，防止 0 秒租约。
	leaseSeconds := int64(leaseDuration / time.Second)
	if leaseSeconds < 1 {
		leaseSeconds = 1
	}

	// 循环逐个给事件更新租约令牌
	// 注意：当前代码行为：只要循环中任意一条更新失败，整个 Claim 直接报错，defer 触发 Rollback，本次所有已经处理的事件全部作废，一条都不返回。
	// 缺点：一批 10 条，第 8 条冲突，前面 7 条已经处理的也要全部放弃，本次认领颗粒度是整批原子。
	for i := range events {
		// 生成唯一租约 token。
		// 上层业务处理这条消息的时候，后续完成 / 释放租约，需要带上这个 token；数据库可以校验：只有持有正确 token，才允许更新这条记录，防止别的 worker 乱操作。
		events[i].LeaseToken = uuid.NewString()
		// tx.ExecContext：在当前事务中执行更新 SQL `ClaimOutboxEventSQL`
		result, execErr := tx.ExecContext(ctx, SQLQueriesPackage.ClaimOutboxEventSQL, events[i].LeaseToken, leaseSeconds, events[i].ID)
		if execErr != nil {
			return nil, fmt.Errorf("lease outbox event %d: %w", events[i].ID, execErr)
		}
		/** RowsAffected() 告诉这个 SQL 到底影响了多少行校验，这里表示必须正好更新 1 行
		 - 如果更新 0 行：说明在本事务查询之后、更新之前，这条记录已经被别的 worker 抢占走了；
		 - 直接返回错误，整个事务回滚，本次一条都不认领。
		 - 这是防御逻辑：虽然前面`FetchPendingTx FOR UPDATE`行锁，但是部分边界场景做双重保险。
		 - 这个if判断表示我期望这个 SQL 恰好修改一条记录
		*/ 
		if affected, affectedErr := result.RowsAffected(); affectedErr != nil || affected != 1 {
			return nil, fmt.Errorf("lease outbox event %d affected %d rows: %w", events[i].ID, affected, affectedErr)
		}
	}
	// 全部循环更新无报错，提交事务。提交成功之后，数据库里面这批 outbox 事件就带上了租约 token 和租约过期时间，其他 worker 无法认领。返回事件列表交给上层去发送消息。
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
