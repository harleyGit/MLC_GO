package send

import (
	queries "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestHGQueueKafkaOnceOutboxAndTaskSameTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationKafkaReadySQL)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "next_index", "generation", "recipients"}).
			AddRow(42, 1, 2, 2))
	mock.ExpectExec(regexp.QuoteMeta(queries.HGNotificationKafkaQueueSQL)).
		WithArgs(uint64(3), uint64(42), uint64(2), 1).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(queries.InsertOutboxEventSQL)).
		WithArgs(sqlmock.AnyArg(), "notification.send.ready.v1", "42", "mlc.notification.send.ready.v1", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(61, 1))
	mock.ExpectExec(regexp.QuoteMeta(queries.HGNotificationKafkaBindOutboxSQL)).
		WithArgs(int64(61), uint64(42), uint64(3)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	queued, err := HGNewRepository(db).hgQueueKafkaOnce(context.Background(), 0)
	if err != nil || !queued {
		t.Fatalf("任务与 Outbox 应原子提交: %t %v", queued, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGQueueKafkaOnceOutboxFailureRollsBack(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationKafkaReadySQL)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "next_index", "generation", "recipients"}).
			AddRow(42, 0, 0, 2))
	mock.ExpectExec(regexp.QuoteMeta(queries.HGNotificationKafkaQueueSQL)).
		WithArgs(uint64(1), uint64(42), uint64(0), 0).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(queries.InsertOutboxEventSQL)).
		WillReturnError(errors.New("Outbox 不可用"))
	mock.ExpectRollback()
	if queued, err := HGNewRepository(db).hgQueueKafkaOnce(context.Background(), 0); err == nil || queued {
		t.Fatalf("Outbox 失败不得留下已排队任务: %t %v", queued, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGQueueKafkaOnceIgnoresNotDueTasks(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	for _, query := range []string{queries.HGNotificationKafkaReadySQL,
		queries.HGNotificationKafkaQueuedExpiredSQL, queries.HGNotificationKafkaLeasedExpiredSQL} {
		mock.ExpectQuery(regexp.QuoteMeta(query)).WillReturnError(sql.ErrNoRows)
	}
	mock.ExpectRollback()
	if queued, err := HGNewRepository(db).hgQueueKafkaOnce(context.Background(), 0); err != nil || queued {
		t.Fatalf("计划时间前不得排队: %t %v", queued, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
