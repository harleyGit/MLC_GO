package VideoInteractionRepositoryPackage

import (
	InteractionConsumerPackage "MLC_GO/internal/consumer/interaction"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
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
