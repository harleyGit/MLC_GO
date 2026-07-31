package VideoInteractionRepositoryPackage

import (
	InteractionConsumerPackage "MLC_GO/internal/consumer/interaction"
	"MLC_GO/internal/events"
	InteractionEventsPackage "MLC_GO/internal/events/interaction"
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

func TestSubmitCoinDebitsWalletWritesLedgerAndOutboxInOneTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertCoinCommandSQL)).
		WithArgs("user-1", "request-1", "submission-1", 2).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.EnsureCoinWalletSQL)).
		WithArgs("user-1").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectCoinWalletForUpdateSQL)).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(10))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectCompletedCoinQuantitySQL)).
		WithArgs("user-1", "submission-1").
		WillReturnRows(sqlmock.NewRows([]string{"quantity"}).AddRow(0))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.DebitCoinWalletSQL)).
		WithArgs(2, "user-1", 2).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertCoinLedgerSQL)).
		WithArgs("user-1", "request-1", "submission-1", -2, 8).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.CompleteCoinCommandSQL)).
		WithArgs("request-1", "user-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
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
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertCoinCommandSQL)).
		WithArgs("user-1", "request-1", "submission-2", 1).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectCoinCommandSQL)).
		WithArgs("user-1", "request-1").
		WillReturnRows(sqlmock.NewRows([]string{"submission_id", "quantity", "status"}).AddRow("submission-1", 2, "completed"))
	mock.ExpectRollback()

	repo := NewRepository(db)
	event := InteractionEventsPackage.VideoInteractionChangedEvent{ActorUserID: "user-1", SubmissionID: "submission-2", Action: "coin", Quantity: 1}
	_, err = repo.SubmitCoin(context.Background(), "request-1", event)
	if !errors.Is(err, ErrCoinIdempotencyConflict) {
		t.Fatalf("error = %v, want ErrCoinIdempotencyConflict", err)
	}
}
