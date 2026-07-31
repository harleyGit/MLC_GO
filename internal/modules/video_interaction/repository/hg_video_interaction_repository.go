package VideoInteractionRepositoryPackage

import (
	InteractionConsumerPackage "MLC_GO/internal/consumer/interaction"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"hash/fnv"
)

const hgInteractionStatShardCount = 64

// Repository 把 Kafka 互动事件幂等写入关系表和分片统计表。
type Repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

func (r *Repository) ApplyEvent(ctx context.Context, event InteractionConsumerPackage.PersistedEvent) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("begin interaction transaction: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, SQLQueriesPackage.InsertInteractionInboxSQL,
		event.EventID, event.EventName, event.EventKey, event.KafkaTopic, event.KafkaPartition, event.KafkaOffset, event.Payload)
	if err != nil {
		return fmt.Errorf("insert interaction inbox: %w", err)
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read interaction inbox result: %w", err)
	}
	if inserted == 0 {
		return tx.Commit()
	}
	switch event.EventName {
	case "video.interaction.changed":
		err = hgApplyVideoInteraction(ctx, tx, event)
	case "user.follow.changed":
		err = hgApplyFollow(ctx, tx, event)
	}
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit interaction transaction: %w", err)
	}
	return nil
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
