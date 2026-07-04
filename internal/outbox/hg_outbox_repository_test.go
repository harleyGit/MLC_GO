package outbox

import (
	"MLC_GO/internal/events"
	VideoEventsPackage "MLC_GO/internal/events/video"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestRepositorySaveStoresDomainEventEnvelope(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertOutboxEventSQL)).
		WithArgs(sqlmock.AnyArg(), VideoEventsPackage.VideoReviewedEventName, "submission_1", "mlc.domain.events", sqlmock.AnyArg()).
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
