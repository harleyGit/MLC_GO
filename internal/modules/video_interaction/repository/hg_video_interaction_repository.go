package VideoInteractionRepositoryPackage

import (
	InteractionConsumerPackage "MLC_GO/internal/consumer/interaction"
	InteractionEventsPackage "MLC_GO/internal/events/interaction"
	CoinRepositoryPackage "MLC_GO/internal/modules/coin/repository"
	CoinServicePackage "MLC_GO/internal/modules/coin/service"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"reflect"
	"time"
)

const (
	hgInteractionStatShardCount = 64

	// hgInteractionBatchSize 分批缩量
	hgInteractionBatchSize = 100
)

var (
	ErrInboxConflict           = errors.New("interaction inbox conflict")
	ErrInsufficientCoinBalance = errors.New("insufficient coin balance")
	ErrCoinLimitExceeded       = errors.New("coin limit exceeded")
	ErrCoinIdempotencyConflict = errors.New("coin idempotency conflict")
)

// Repository 把 Kafka 互动事件幂等写入关系表和分片统计表。
type Repository struct {
	db    *sql.DB
	topic string
}

func NewRepository(db *sql.DB) *Repository { return NewRepositoryWithTopic(db, "mlc.domain.events") }

// NewRepositoryWithTopic 创建 Interaction repository，并让投币 Outbox 遵循当前环境业务 topic。
func NewRepositoryWithTopic(db *sql.DB, topic string) *Repository {
	return &Repository{db: db, topic: topic}
}

/* 执行流程
Kafka 消费到事件
       |
       v
ApplyEvent() / ApplyEvents()
       |
       v
按照 hgInteractionBatchSize 分批
       |
       v
hgApplyBatch()
       |
       +-- 开启数据库事务 BeginTx
       |
       +-- 遍历当前批次事件
       |      |
       |      +-- hgClaimInbox() 检查重复
       |      |      |
       |      |      +-- 重复 → 跳过
       |      |      |
       |      |      +-- 未重复 → 处理
       |      |
       |      +-- 点赞事件 → hgApplyVideoInteraction()
       |      |
       |      +-- 关注事件 → hgApplyFollow()
       |
       +-- 成功 → Commit() 提交
       |
       +-- 失败 → Rollback() 回滚
       |
       v
hgObservePersistenceBatch()
记录批次数、事件数、耗时、失败次数
*/
// ApplyEvent 将 Kafka 消费到的用户交互事件，分批写入数据库，同时保证事务安全、避免重复处理，并记录性能指标
// 单个事件路口
// 例如：用户点赞视频、取消点赞、关注用户、取消关注等操作。
// ApplyEvent()：处理单个事件
//	@param ctx
//	@param event
//	@return error
func (r *Repository) ApplyEvent(ctx context.Context, event InteractionConsumerPackage.PersistedEvent) error {
	// 作用：把一个事件包装成切片，交给批量处理方法。
	// 表示创建一个只包含 event 的切片
	// 复用批量处理逻辑，不需要单独实现数据库写入
	return r.ApplyEvents(ctx, []InteractionConsumerPackage.PersistedEvent{event})
}

// ApplyEvents 按固定上限拆分事务，减少逐条开事务开销且避免大事务长时间持锁。
// 分批处理事件
//
//	@param ctx
//	@param batch
//	@return error
func (r *Repository) ApplyEvents(ctx context.Context, batch []InteractionConsumerPackage.PersistedEvent) error {
	for start := 0; start < len(batch); start += hgInteractionBatchSize {
		end := start + hgInteractionBatchSize
		if end > len(batch) {
			end = len(batch)
		}

		// startedAt 记录开始时间
		startedAt := time.Now()

		// 真正执行数据库写入
		err := r.hgApplyBatch(ctx, batch[start:end])
		// 记录监控指标
		hgObservePersistenceBatch(len(batch[start:end]), time.Since(startedAt), err)
		// 若是某一批失败，立即停止，不再处理后面的批次
		if err != nil {
			return err
		}
	}
	return nil
}

// hgApplyBatch 数据库事务处理（核心），负责分批调度
// 负责事务、去重、业务执行和提交。
//
//	@param ctx
//	@param batch
//	@return error
func (r *Repository) hgApplyBatch(ctx context.Context, batch []InteractionConsumerPackage.PersistedEvent) error {

	// BeginTx()：开启数据库事务。
	// LevelReadCommitted：读已提交隔离级别，避免读取其他事务尚未提交的数据。
	// tx：事务对象，后续数据库操作都通过它执行；如果前面已经 Commit() 成功，后续 Rollback() 不会撤销已提交的数据。
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("begin interaction transaction: %w", err)
	}
	// 函数推出时尝试回滚事务
	defer tx.Rollback()
	for _, event := range batch {
		// 检查事件是否重复，从调用方式看，负责事件去重或幂等登记
		duplicate, applyErr := hgClaimInbox(ctx, tx, event)
		if applyErr != nil {
			return applyErr
		}
		if duplicate {
			continue
		}
		switch event.EventName {
		case "video.interaction.changed": // 点赞事件
			applyErr = hgApplyVideoInteraction(ctx, tx, event)
		case "user.follow.changed": // 关注事件
			applyErr = hgApplyFollow(ctx, tx, event)
		}
		// 函数立即返回错误，触发事务回滚。
		if applyErr != nil {
			return applyErr
		}
	}

	// 提交事务
	// 	当前批次所有事件成功后，提交事务。
	// 例如一批有 100 条：
	// - 100 条全部成功 → Commit()。
	// - 第 60 条失败 → Rollback()，本批次数据库修改全部撤销。
	// 注意：每一批是一个独立事务，不是所有批次共用一个事务。 如果第 2 批失败，第 1 批已经提交的数据不会回滚。
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit interaction transaction: %w", err)
	}
	return nil
}

/*执行流程
Kafka 消费到事件
        |
        v
hgClaimInbox()
        |
        v
尝试 INSERT 到 Inbox 表
        |
        +-- 插入成功（RowsAffected = 1）
        |       |
        |       +-- 返回 false, nil
        |           表示新事件，可以继续处理
        |
        +-- 未插入（RowsAffected != 1）
                |
                v
        根据 EventID 查询
                |
                +-- 找到记录
                |      |
                |      +-- 内容相同 → 重复事件
                |      |                返回 true, nil
                |      |
                |      +-- 内容不同 → 事件冲突
                |                       返回 error
                |
                +-- 未找到记录
                       |
                       v
                根据 Kafka 投递位置查询
                       |
                       +-- 找到 → 冲突
                       |
                       +-- 查询失败 → 返回 error

为什么需要两种去重方式？
去重方式	        判断依据	                    解决的问题
业务事件去重	           EventID	                同一个业务事件被重复投递
Kafka 投递位置检查	Topic + Partition + Offset	同一个 Kafka 消息位置出现不一致的事件身份
例如同一个点赞事件重新投递到 Kafka，可能获得不同的 Offset，但 EventID 保持不变，因此仅依靠 Kafka Offset 无法完成业务幂等。

*/
// hgClaimInbox 判断 Kafka 消费到的事件是否已经处理过，避免重复执行点赞、关注等业务操作，同时识别异常冲突。
// 返回值的几种组合方式表示的意义：
// false, nil	    含义：新事件	后续处理：执行业务逻辑
// true, nil	    含义：重复事件	跳过行业务逻辑
// true, error	    含义：数据库错误或者事件冲突	终止当前批次并回滚事务
func hgClaimInbox(ctx context.Context, tx *sql.Tx, event InteractionConsumerPackage.PersistedEvent) (bool, error) {

	// 尝试插入 Inbox 表
	result, err := tx.ExecContext(ctx, SQLQueriesPackage.InsertInteractionInboxSQL,
		event.EventID, event.EventName, event.EventKey, event.KafkaTopic, event.KafkaPartition, event.KafkaOffset, event.Payload)
	if err != nil {
		return false, fmt.Errorf("insert interaction inbox: %w", err)
	}
	// 判断是否插入成功
	inserted, err := result.RowsAffected()
	if err != nil {
		// 不是重复事件，可以继续执行点赞、关注等业务操作。
		return false, fmt.Errorf("read interaction inbox result: %w", err)
	}
	// 1 插入成功，表示新事件，可以继续处理； 0 未插入，或者冲突因为唯一键重复
	if inserted == 1 {
		return false, nil
	}
	var existing InteractionConsumerPackage.PersistedEvent
	// 业务事件去重
	err = tx.QueryRowContext(ctx, SQLQueriesPackage.SelectInteractionInboxByEventIDSQL, event.EventID).
		Scan(&existing.EventID, &existing.EventName, &existing.EventKey, &existing.KafkaTopic, &existing.KafkaPartition, &existing.KafkaOffset, &existing.Payload)
	if err == nil {
		// 判断是真正重复，还是数据冲突
		if existing.EventName == event.EventName && existing.EventKey == event.EventKey && hgJSONEqual(existing.Payload, event.Payload) {
			hgObserveInboxDuplicate()
			//  true 表示重复事件，已经处理过了，不需要再执行点赞、关注等业务操作。
			return true, nil
		}
		hgObserveInboxConflict()
		// 这属于事件 ID 冲突，不能简单当成重复事件忽略。
		return false, fmt.Errorf("%w: event_id=%s topic=%s partition=%d offset=%d", ErrInboxConflict, event.EventID, event.KafkaTopic, event.KafkaPartition, event.KafkaOffset)
	}
	// 判断没有查询到记录
	// sql.ErrNoRows 表示 SQL 查询没有找到记录。
	// 这里的逻辑是：
	// - 如果没有找到记录，继续往下执行。
	// - 如果是其他数据库错误，立即返回。
	if !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("select interaction inbox by event id: %w", err)
	}

	// 根据 Kafka 投递位置再次查询【Kafka投递位置检查】
	// 为啥再次查询？
	// 因为一条 Kafka 消息的位置由三个字段共同确定：Topic + Partition + Offset
	/*
		Topic     = interaction-events
		Partition = 2
		Offset    = 1050
	*/
	// 在同一个 Kafka Topic 分区中，同一个 Offset 只能对应一个消息位置。
	// 若是数据库中存在EventID = 1001、EventID = 1002的记录，那么这两个记录的Kafka的Offset、Topic、Partition都相同，也就是说两个事件 ID 不同，但 Kafka 投递位置相同，属于异常冲突。
	// QueryRowContext()：根据 EventID 查询数据库中的一条记录。
	// Scan()：将查询结果赋值给 existing 结构体。
	err = tx.QueryRowContext(ctx, SQLQueriesPackage.SelectInteractionInboxByDeliverySQL, event.KafkaTopic, event.KafkaPartition, event.KafkaOffset).
		Scan(&existing.EventID, &existing.EventName, &existing.EventKey, &existing.KafkaTopic, &existing.KafkaPartition, &existing.KafkaOffset, &existing.Payload)
	if err != nil {
		return false, fmt.Errorf("select interaction inbox by delivery: %w", err)
	}
	hgObserveInboxConflict()
	return false, fmt.Errorf("%w: event_id=%s topic=%s partition=%d offset=%d", ErrInboxConflict, event.EventID, event.KafkaTopic, event.KafkaPartition, event.KafkaOffset)
}

func hgJSONEqual(left string, right string) bool {
	var leftValue any
	var rightValue any
	if json.Unmarshal([]byte(left), &leftValue) != nil || json.Unmarshal([]byte(right), &rightValue) != nil {
		return left == right
	}
	return reflect.DeepEqual(leftValue, rightValue)
}

// SubmitCoin 在一个 MySQL 事务内完成幂等占位、资产扣减、不可变流水和 Outbox 写入。
func (r *Repository) SubmitCoin(ctx context.Context, requestID string, event InteractionEventsPackage.VideoInteractionChangedEvent) (bool, error) {
	result, err := CoinServicePackage.NewHGService(CoinRepositoryPackage.NewHGRepository(r.db, r.topic)).Debit(ctx, CoinServicePackage.HGDebitCommand{
		UserID: event.ActorUserID, RequestID: requestID, Amount: uint64(event.Quantity), Reason: "video_coin",
		BusinessType: "video_coin", BusinessKey: event.SubmissionID, BusinessLimit: 2, Event: event,
	})
	if errors.Is(err, CoinRepositoryPackage.ErrHGInsufficientBalance) {
		return false, ErrInsufficientCoinBalance
	}
	if errors.Is(err, CoinRepositoryPackage.ErrHGBusinessLimit) {
		return false, ErrCoinLimitExceeded
	}
	if errors.Is(err, CoinRepositoryPackage.ErrHGIdempotencyConflict) {
		return false, ErrCoinIdempotencyConflict
	}
	if err != nil {
		return false, err
	}
	if result.Committed {
		hgObserveWalletDebit()
	}
	return result.Committed, nil
}

func hgApplyVideoInteraction(ctx context.Context, tx *sql.Tx, event InteractionConsumerPackage.PersistedEvent) error {
	if event.Action == "share" {
		result, err := tx.ExecContext(ctx, SQLQueriesPackage.InsertShareRecordSQL, event.EventID, event.ActorUserID, event.SubmissionID)
		if err != nil {
			return fmt.Errorf("insert share record: %w", err)
		}
		inserted, _ := result.RowsAffected()
		if inserted == 1 {
			return hgIncrementVideoStat(ctx, tx, event.SubmissionID, hgShard(event.ActorUserID), 0, 0, 0, 1)
		}
		return nil
	}
	var oldActive bool
	var oldQuantity int
	err := tx.QueryRowContext(ctx, SQLQueriesPackage.SelectVideoInteractionForUpdateSQL, event.ActorUserID, event.SubmissionID, event.Action).Scan(&oldActive, &oldQuantity)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("select video interaction: %w", err)
	}
	newActive, newQuantity := event.Active, 0
	if event.Action == "coin" {
		newActive = true
		newQuantity = oldQuantity + event.Quantity
		if newQuantity > 2 {
			newQuantity = 2
		}
	}
	if errors.Is(err, sql.ErrNoRows) {
		_, err = tx.ExecContext(ctx, SQLQueriesPackage.InsertVideoInteractionSQL, event.ActorUserID, event.SubmissionID, event.Action, newActive, newQuantity)
	} else {
		_, err = tx.ExecContext(ctx, SQLQueriesPackage.UpdateVideoInteractionSQL, newActive, newQuantity, event.ActorUserID, event.SubmissionID, event.Action)
	}
	if err != nil {
		return fmt.Errorf("save video interaction: %w", err)
	}
	delta := 0
	if event.Action == "coin" {
		delta = newQuantity - oldQuantity
	} else if oldActive != newActive {
		if newActive {
			delta = 1
		} else {
			delta = -1
		}
	}
	if delta == 0 {
		return nil
	}
	switch event.Action {
	case "like":
		return hgIncrementVideoStat(ctx, tx, event.SubmissionID, hgShard(event.ActorUserID), delta, 0, 0, 0)
	case "coin":
		return hgIncrementVideoStat(ctx, tx, event.SubmissionID, hgShard(event.ActorUserID), 0, delta, 0, 0)
	case "favorite":
		return hgIncrementVideoStat(ctx, tx, event.SubmissionID, hgShard(event.ActorUserID), 0, 0, delta, 0)
	}
	return nil
}

func hgIncrementVideoStat(ctx context.Context, tx *sql.Tx, submissionID string, shard int, like int, coin int, favorite int, share int) error {
	_, err := tx.ExecContext(ctx, SQLQueriesPackage.UpsertVideoInteractionStatShardSQL, submissionID, shard, like, coin, favorite, share)
	if err != nil {
		return fmt.Errorf("update video interaction stat shard: %w", err)
	}
	return nil
}

func hgApplyFollow(ctx context.Context, tx *sql.Tx, event InteractionConsumerPackage.PersistedEvent) error {
	var oldFollowID sql.NullString
	var oldActive bool
	err := tx.QueryRowContext(ctx, SQLQueriesPackage.SelectFollowForUpdateSQL, event.FollowerID, event.FolloweeID).Scan(&oldFollowID, &oldActive)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("select follow relation: %w", err)
	}
	if errors.Is(err, sql.ErrNoRows) {
		_, err = tx.ExecContext(ctx, SQLQueriesPackage.InsertFollowSQL, event.FollowID, event.FollowerID, event.FolloweeID, event.Active)
	} else {
		_, err = tx.ExecContext(ctx, SQLQueriesPackage.UpdateFollowSQL, event.FollowID, event.Active, event.FollowerID, event.FolloweeID)
	}
	if err != nil {
		return fmt.Errorf("save follow relation: %w", err)
	}
	if oldActive == event.Active {
		return nil
	}
	delta := -1
	if event.Active {
		delta = 1
	}
	_, err = tx.ExecContext(ctx, SQLQueriesPackage.UpsertFollowStatShardSQL, event.FolloweeID, hgShard(event.FollowerID), delta)
	if err != nil {
		return fmt.Errorf("update follow stat shard: %w", err)
	}
	return nil
}

func hgShard(value string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(value))
	return int(h.Sum32() % hgInteractionStatShardCount)
}
