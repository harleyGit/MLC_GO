package outbox

import (
	"MLC_GO/internal/events"
	VideoEventsPackage "MLC_GO/internal/events/video"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql/driver"
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

type hgCaptureEventIDArgument struct{}

func (hgCaptureEventIDArgument) Match(value driver.Value) bool {
	eventID, ok := value.(string)
	if !ok || eventID == "" {
		return false
	}
	hgExpectedOutboxEventID = eventID
	return true
}

func TestRepositoryClaimCommitsLeaseBeforeReturningEvents(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	rows := sqlmock.NewRows([]string{"id", "event_id", "event_name", "event_key", "topic", "payload", "retry_count"}).
		AddRow(int64(1), "event-1", "video.published", "submission-1", "events", []byte(`{}`), 0)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectPendingOutboxEventsSQL)).WithArgs(8).WillReturnRows(rows)
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.ClaimOutboxEventSQL)).
		WithArgs(sqlmock.AnyArg(), int64(30), int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	events, err := NewRepository(db, "events").Claim(context.Background(), 8, 30*time.Second)
	if err != nil {
		t.Fatalf("Claim() error = %v", err)
	}
	if len(events) != 1 || events[0].LeaseToken == "" {
		t.Fatalf("events = %#v, want one leased event", events)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

type hgEnvelopeEventIDArgument struct{}

func (hgEnvelopeEventIDArgument) Match(value driver.Value) bool {
	payload, ok := value.([]byte)
	if !ok {
		return false
	}
	var envelope events.EventEnvelope
	if json.Unmarshal(payload, &envelope) != nil {
		return false
	}
	return envelope.EventID != "" && envelope.EventID == hgExpectedOutboxEventID
}

var hgExpectedOutboxEventID string

func TestRepositorySaveStoresDomainEventEnvelope(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	hgExpectedOutboxEventID = ""
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertOutboxEventSQL)).
		WithArgs(hgCaptureEventIDArgument{}, VideoEventsPackage.VideoReviewedEventName, "submission_1", "mlc.domain.events", hgEnvelopeEventIDArgument{}).
		WillReturnResult(sqlmock.NewResult(1, 1))

	repo := NewRepository(db, "mlc.domain.events")
	err = repo.Save(context.Background(), VideoEventsPackage.VideoReviewedEvent{
		EventMeta:    events.NewEventMeta(context.Background()),
		SubmissionID: "submission_1",
		UserID:       "user_1",
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}
