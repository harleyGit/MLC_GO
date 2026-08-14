package VideoInteractionRepositoryPackage

import (
	InteractionConsumerPackage "MLC_GO/internal/consumer/interaction"
	"MLC_GO/internal/events"
	InteractionEventsPackage "MLC_GO/internal/events/interaction"
	CoinModelPackage "MLC_GO/internal/modules/coin/model"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestApplyEventCommitsDuplicateInboxWithoutApplyingAgain(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertInteractionInboxSQL)).
		WithArgs("event-1", "video.interaction.changed", "key-1", "mlc.domain.events", int32(1), int64(9), `{"action":"like"}`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectInteractionInboxByEventIDSQL)).
		WithArgs("event-1").
		WillReturnRows(sqlmock.NewRows([]string{"event_id", "event_name", "event_key", "kafka_topic", "kafka_partition", "kafka_offset", "payload"}).
			AddRow("event-1", "video.interaction.changed", "key-1", "mlc.domain.events", int32(1), int64(9), `{"action":"like"}`))
	mock.ExpectCommit()

	repo := NewRepository(db)
	err = repo.ApplyEvent(context.Background(), InteractionConsumerPackage.PersistedEvent{
		EventID: "event-1", EventName: "video.interaction.changed", EventKey: "key-1",
		KafkaTopic: "mlc.domain.events", KafkaPartition: 1, KafkaOffset: 9, Payload: `{"action":"like"}`,
	})
	if err != nil {
		t.Fatalf("ApplyEvent() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestApplyEventsCommitsOneBoundedBatch(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	for offset := int64(9); offset <= 10; offset++ {
		mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertInteractionInboxSQL)).
			WithArgs("event-1", "video.interaction.changed", "key-1", "mlc.domain.events", int32(1), offset, `{"action":"like"}`).
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectInteractionInboxByEventIDSQL)).
			WithArgs("event-1").
			WillReturnRows(sqlmock.NewRows([]string{"event_id", "event_name", "event_key", "kafka_topic", "kafka_partition", "kafka_offset", "payload"}).
				AddRow("event-1", "video.interaction.changed", "key-1", "mlc.domain.events", int32(1), int64(9), `{"action":"like"}`))
	}
	mock.ExpectCommit()

	repo := NewRepository(db)
	event := InteractionConsumerPackage.PersistedEvent{EventID: "event-1", EventName: "video.interaction.changed", EventKey: "key-1", KafkaTopic: "mlc.domain.events", KafkaPartition: 1, Payload: `{"action":"like"}`}
	event.KafkaOffset = 9
	second := event
	second.KafkaOffset = 10
	if err := repo.ApplyEvents(context.Background(), []InteractionConsumerPackage.PersistedEvent{event, second}); err != nil {
		t.Fatalf("ApplyEvents() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestJSONEqualIgnoresObjectKeyOrder(t *testing.T) {
	if !hgJSONEqual(`{"action":"like","active":true}`, `{"active":true,"action":"like"}`) {
		t.Fatal("semantically equal JSON payloads must match")
	}
}

func TestApplyFollowPersistsBusinessIDAndUpdatesCount(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertInteractionInboxSQL)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectFollowForUpdateSQL)).
		WithArgs("user-1", "user-2").
		WillReturnRows(sqlmock.NewRows([]string{"relation_id", "active"}))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertFollowSQL)).
		WithArgs("O123", "user-1", "user-2", true).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.UpsertFollowStatShardSQL)).
		WithArgs("user-2", hgShard("user-1"), 1).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	event := InteractionConsumerPackage.PersistedEvent{
		EventID: "event-follow-1", EventName: "user.follow.changed", EventKey: "user-2:user-1:follow",
		FollowID: "O123", FollowerID: "user-1", FolloweeID: "user-2", Active: true,
		KafkaTopic: "mlc.domain.events", KafkaPartition: 0, KafkaOffset: 1, Payload: `{"followId":"O123"}`,
	}
	if err := NewRepository(db).ApplyEvent(context.Background(), event); err != nil {
		t.Fatalf("ApplyEvent() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestSubmitCoinDebitsWalletWritesLedgerAndOutboxInOneTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.EnsureCoinWalletSQL)).
		WithArgs("user-1").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectCoinWalletForUpdateSQL)).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(10))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectLegacyCoinCommandSQL)).
		WithArgs("user-1", "request-1").
		WillReturnRows(sqlmock.NewRows([]string{"submission_id", "quantity", "status"}))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertCoinRequestSQL)).
		WithArgs("user-1", "request-1", CoinModelPackage.HGOperationDebit, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectCoinBusinessDebitTotalSQL)).
		WithArgs("user-1", "video_coin", "submission-1", "video_coin", "user-1", "submission-1").
		WillReturnRows(sqlmock.NewRows([]string{"quantity"}).AddRow(0))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectCoinLotsForDebitSQL)).WillReturnRows(sqlmock.NewRows([]string{"id", "remaining_amount", "expires_at"}).AddRow(1, 10, nil))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.DebitCoinWalletSQL)).
		WithArgs(uint64(2), "user-1", uint64(2)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertCoinTransactionSQL)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.CompleteCoinRequestSQL)).
		WithArgs(uint64(1), uint64(8), "user-1", "request-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.UpdateCoinLotRemainingSQL)).WithArgs(uint64(8), uint64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertCoinAllocationSQL)).WithArgs(uint64(1), uint64(1), uint64(2), "debit").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO outbox_events").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	repo := NewRepository(db)
	event := InteractionEventsPackage.VideoInteractionChangedEvent{EventMeta: events.NewEventMeta(context.Background()), ActorUserID: "user-1", SubmissionID: "submission-1", Action: "coin", Active: true, Quantity: 2}
	if _, err := repo.SubmitCoin(context.Background(), "request-1", event); err != nil {
		t.Fatalf("SubmitCoin() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestSubmitCoinRejectsRequestIDReusedForDifferentCommand(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.EnsureCoinWalletSQL)).
		WithArgs("user-1").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectCoinWalletForUpdateSQL)).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(8))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectLegacyCoinCommandSQL)).
		WithArgs("user-1", "request-1").
		WillReturnRows(sqlmock.NewRows([]string{"submission_id", "quantity", "status"}))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertCoinRequestSQL)).
		WithArgs("user-1", "request-1", CoinModelPackage.HGOperationDebit, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectCoinRequestSQL)).
		WithArgs("user-1", "request-1").
		WillReturnRows(sqlmock.NewRows([]string{"operation", "command_hash", "status", "transaction_id", "balance_after"}).AddRow("debit", "different", "completed", 1, 8))
	mock.ExpectRollback()

	repo := NewRepository(db)
	event := InteractionEventsPackage.VideoInteractionChangedEvent{ActorUserID: "user-1", SubmissionID: "submission-2", Action: "coin", Quantity: 1}
	_, err = repo.SubmitCoin(context.Background(), "request-1", event)
	if !errors.Is(err, ErrCoinIdempotencyConflict) {
		t.Fatalf("error = %v, want ErrCoinIdempotencyConflict", err)
	}
}
