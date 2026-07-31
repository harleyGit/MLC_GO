package VideoInteractionRepositoryPackage

import (
	InteractionConsumerPackage "MLC_GO/internal/consumer/interaction"
	InteractionEventsPackage "MLC_GO/internal/events/interaction"
	"MLC_GO/internal/outbox"
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
	hgInteractionBatchSize      = 100
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

func (r *Repository) ApplyEvent(ctx context.Context, event InteractionConsumerPackage.PersistedEvent) error {
	return r.ApplyEvents(ctx, []InteractionConsumerPackage.PersistedEvent{event})
}

// ApplyEvents 按固定上限拆分事务，减少逐条开事务开销且避免大事务长时间持锁。
func (r *Repository) ApplyEvents(ctx context.Context, batch []InteractionConsumerPackage.PersistedEvent) error {
	for start := 0; start < len(batch); start += hgInteractionBatchSize {
		end := start + hgInteractionBatchSize
		if end > len(batch) {
			end = len(batch)
		}
		startedAt := time.Now()
		err := r.hgApplyBatch(ctx, batch[start:end])
		hgObservePersistenceBatch(len(batch[start:end]), time.Since(startedAt), err)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) hgApplyBatch(ctx context.Context, batch []InteractionConsumerPackage.PersistedEvent) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("begin interaction transaction: %w", err)
	}
	defer tx.Rollback()
	for _, event := range batch {
		duplicate, applyErr := hgClaimInbox(ctx, tx, event)
		if applyErr != nil {
			return applyErr
		}
		if duplicate {
			continue
		}
		switch event.EventName {
		case "video.interaction.changed":
			applyErr = hgApplyVideoInteraction(ctx, tx, event)
		case "user.follow.changed":
			applyErr = hgApplyFollow(ctx, tx, event)
		}
		if applyErr != nil {
			return applyErr
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit interaction transaction: %w", err)
	}
	return nil
}

func hgClaimInbox(ctx context.Context, tx *sql.Tx, event InteractionConsumerPackage.PersistedEvent) (bool, error) {
	result, err := tx.ExecContext(ctx, SQLQueriesPackage.InsertInteractionInboxSQL,
		event.EventID, event.EventName, event.EventKey, event.KafkaTopic, event.KafkaPartition, event.KafkaOffset, event.Payload)
	if err != nil {
		return false, fmt.Errorf("insert interaction inbox: %w", err)
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("read interaction inbox result: %w", err)
	}
	if inserted == 1 {
		return false, nil
	}
	var existing InteractionConsumerPackage.PersistedEvent
	err = tx.QueryRowContext(ctx, SQLQueriesPackage.SelectInteractionInboxByEventIDSQL, event.EventID).
		Scan(&existing.EventID, &existing.EventName, &existing.EventKey, &existing.KafkaTopic, &existing.KafkaPartition, &existing.KafkaOffset, &existing.Payload)
	if err == nil {
		if existing.EventName == event.EventName && existing.EventKey == event.EventKey && hgJSONEqual(existing.Payload, event.Payload) {
			hgObserveInboxDuplicate()
			return true, nil
		}
		hgObserveInboxConflict()
		return false, fmt.Errorf("%w: event_id=%s topic=%s partition=%d offset=%d", ErrInboxConflict, event.EventID, event.KafkaTopic, event.KafkaPartition, event.KafkaOffset)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("select interaction inbox by event id: %w", err)
	}
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
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return false, fmt.Errorf("begin coin transaction: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, SQLQueriesPackage.InsertCoinCommandSQL, event.ActorUserID, requestID, event.SubmissionID, event.Quantity)
	if err != nil {
		return false, fmt.Errorf("insert coin command: %w", err)
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("read coin command result: %w", err)
	}
	if inserted == 0 {
		var submissionID string
		var quantity int
		var status string
		if err := tx.QueryRowContext(ctx, SQLQueriesPackage.SelectCoinCommandSQL, event.ActorUserID, requestID).Scan(&submissionID, &quantity, &status); err != nil {
			return false, fmt.Errorf("select duplicate coin command: %w", err)
		}
		if submissionID != event.SubmissionID || quantity != event.Quantity || status != "completed" {
			return false, ErrCoinIdempotencyConflict
		}
		return false, tx.Commit()
	}
	if _, err := tx.ExecContext(ctx, SQLQueriesPackage.EnsureCoinWalletSQL, event.ActorUserID); err != nil {
		return false, fmt.Errorf("ensure coin wallet: %w", err)
	}
	var currentBalance uint64
	if err := tx.QueryRowContext(ctx, SQLQueriesPackage.SelectCoinWalletForUpdateSQL, event.ActorUserID).Scan(&currentBalance); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, ErrInsufficientCoinBalance
		}
		return false, fmt.Errorf("lock coin wallet: %w", err)
	}
	var completedQuantity int
	if err := tx.QueryRowContext(ctx, SQLQueriesPackage.SelectCompletedCoinQuantitySQL, event.ActorUserID, event.SubmissionID).Scan(&completedQuantity); err != nil {
		return false, fmt.Errorf("select completed coin quantity: %w", err)
	}
	if completedQuantity+event.Quantity > 2 {
		return false, ErrCoinLimitExceeded
	}
	result, err = tx.ExecContext(ctx, SQLQueriesPackage.DebitCoinWalletSQL, event.Quantity, event.ActorUserID, event.Quantity)
	if err != nil {
		return false, fmt.Errorf("debit coin wallet: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("read coin debit result: %w", err)
	}
	if affected != 1 {
		return false, ErrInsufficientCoinBalance
	}
	balanceAfter := currentBalance - uint64(event.Quantity)
	if _, err := tx.ExecContext(ctx, SQLQueriesPackage.InsertCoinLedgerSQL, event.ActorUserID, requestID, event.SubmissionID, -event.Quantity, balanceAfter); err != nil {
		return false, fmt.Errorf("insert coin ledger: %w", err)
	}
	if _, err := tx.ExecContext(ctx, SQLQueriesPackage.CompleteCoinCommandSQL, requestID, event.ActorUserID); err != nil {
		return false, fmt.Errorf("complete coin command: %w", err)
	}
	if err := outbox.NewRepository(r.db, r.topic).SaveTx(ctx, tx, event); err != nil {
		return false, fmt.Errorf("save coin outbox: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit coin transaction: %w", err)
	}
	hgObserveWalletDebit()
	return true, nil
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
	var oldActive bool
	err := tx.QueryRowContext(ctx, SQLQueriesPackage.SelectFollowForUpdateSQL, event.FollowerID, event.FolloweeID).Scan(&oldActive)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("select follow relation: %w", err)
	}
	if errors.Is(err, sql.ErrNoRows) {
		_, err = tx.ExecContext(ctx, SQLQueriesPackage.InsertFollowSQL, event.FollowerID, event.FolloweeID, event.Active)
	} else {
		_, err = tx.ExecContext(ctx, SQLQueriesPackage.UpdateFollowSQL, event.Active, event.FollowerID, event.FolloweeID)
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
