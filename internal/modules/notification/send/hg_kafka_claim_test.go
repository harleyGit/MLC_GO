package send

import (
	notificationevents "MLC_GO/internal/events/notification"
	notification "MLC_GO/internal/modules/notification"
	queries "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestHGDispatchReadyIgnoresOldGeneration(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	worker := HGNewWorker(HGNewRepository(db), notification.HGNewService(notification.HGNewRepository(db)))
	event, _ := notificationevents.HGNewSendReadyEvent(context.Background(), 12, 0, 1)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationKafkaClaimSQL)).
		WithArgs(uint64(12), uint64(1), uint16(0)).WillReturnError(sqlmock.ErrCancelled)
	mock.ExpectRollback()
	if err := worker.HGDispatchReady(context.Background(), event); !errors.Is(err, sqlmock.ErrCancelled) {
		t.Fatalf("数据库异常不能当作重复事件提交: %v", err)
	}
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationKafkaClaimSQL)).
		WithArgs(uint64(12), uint64(1), uint16(0)).
		WillReturnRows(sqlmock.NewRows([]string{"template_id", "version", "audience", "variables", "index", "attempts"}))
	mock.ExpectRollback()
	if err := worker.HGDispatchReady(context.Background(), event); err != nil {
		t.Fatalf("已处理或旧代次必须安全跳过: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGDispatchReadyPersistsBeforeKafkaAck(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	worker := HGNewWorker(HGNewRepository(db), notification.HGNewService(notification.HGNewRepository(db)))
	event, _ := notificationevents.HGNewSendReadyEvent(context.Background(), 12, 0, 1)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationKafkaClaimSQL)).
		WithArgs(uint64(12), uint64(1), uint16(0)).
		WillReturnRows(sqlmock.NewRows([]string{"template_id", "version", "audience", "variables", "index", "attempts"}).
			AddRow(7, 2, []byte(`["user_1"]`), []byte(`{"name":"用户A","when":"02:00"}`), 0, 0))
	mock.ExpectExec(regexp.QuoteMeta(queries.HGNotificationKafkaLeaseSQL)).
		WithArgs(sqlmock.AnyArg(), uint64(12), uint64(1), uint16(0)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendBoundVersionSQL)).WithArgs(uint32(2), uint64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"category", "title", "body", "schema", "action", "config", "channels"}).
			AddRow("system", "维护 {{name}}", "您好 {{name}}，维护将在 {{when}} 开始。",
				[]byte(hgVersion().Schema), "none", []byte(`{}`), []byte(`["in_app"]`)))
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendUserExistsSQL)).WithArgs("user_1").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(1))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(queries.HGNotificationLedgerInsertSQL)).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "user_1", "system").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationCounterReadySQL)).
		WillReturnRows(sqlmock.NewRows([]string{"ready"}).AddRow(false))
	mock.ExpectExec(regexp.QuoteMeta(queries.HGNotificationInsertSQL)).
		WithArgs(sqlmock.AnyArg(), "user_1", "system", "维护 用户A", "您好 用户A，维护将在 02:00 开始。", "", "").
		WillReturnResult(sqlmock.NewResult(55, 1))
	mock.ExpectCommit()
	mock.ExpectExec(regexp.QuoteMeta(queries.HGNotificationKafkaAdvanceSQL)).
		WithArgs(1, "done", uint64(12), sqlmock.AnyArg(), uint64(1), 0).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := worker.HGDispatchReady(context.Background(), event); err != nil {
		t.Fatalf("落库并推进后才能确认 Kafka 消息: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGDispatchReadyDoesNotAckWithoutStateReceipt(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	worker := HGNewWorker(HGNewRepository(db), notification.HGNewService(notification.HGNewRepository(db)))
	event, _ := notificationevents.HGNewSendReadyEvent(context.Background(), 12, 0, 1)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationKafkaClaimSQL)).
		WithArgs(uint64(12), uint64(1), uint16(0)).
		WillReturnRows(sqlmock.NewRows([]string{"template_id", "version", "audience", "variables", "index", "attempts"}).
			AddRow(7, 2, []byte(`["user_1"]`), []byte(`{}`), 0, 0))
	mock.ExpectExec(regexp.QuoteMeta(queries.HGNotificationKafkaLeaseSQL)).
		WithArgs(sqlmock.AnyArg(), uint64(12), uint64(1), uint16(0)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendBoundVersionSQL)).
		WillReturnError(errors.New("模板暂不可用"))
	mock.ExpectExec(regexp.QuoteMeta(queries.HGNotificationKafkaRetrySQL)).
		WithArgs("pending", uint8(1), 2, uint64(12), sqlmock.AnyArg(), uint64(1), 0).
		WillReturnError(errors.New("回执写入失败"))
	if err := worker.HGDispatchReady(context.Background(), event); err == nil {
		t.Fatal("DB 未记录重试时不得推进 Kafka offset")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
