package notification

import (
	SQL "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
)

type hgHintPublisher struct {
	user    string          // 收到提示的本人 ID，空串表示未发布。
	payload json.RawMessage // 只用于本地协议断言，不含真实用户。
	calls   int             // 实际发布次数，幂等重放不应增加。
	failure error           // 模拟网关故障；不得影响权威入箱。
}

func (publisher *hgHintPublisher) HGPublishUser(_ context.Context, user string, payload json.RawMessage) error {
	publisher.user, publisher.payload = user, payload
	publisher.calls++
	return publisher.failure
}

// 固定 SQL 序列验证事务提交后才提示，网关失败不使数据库任务重试。
func TestHGNotificationHintAfterCommit(t *testing.T) {
	for _, gatewayFailure := range []bool{false, true} {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		hints := HGNewHints()
		publisher := &hgHintPublisher{}
		if gatewayFailure {
			publisher.failure = errors.New("网关不可用")
		}
		hints.HGUsePublisher(publisher)
		service := HGNewService(HGNewRepository(db), hints)
		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(SQL.HGNotificationLedgerInsertSQL)).
			WithArgs(sqlmock.AnyArg(), "evt-1", "U1", "system").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectQuery(regexp.QuoteMeta(SQL.HGNotificationCounterReadySQL)).
			WillReturnRows(sqlmock.NewRows([]string{"ready"}).AddRow(false))
		mock.ExpectExec(regexp.QuoteMeta(SQL.HGNotificationInsertSQL)).
			WithArgs("evt-1", "U1", "system", "通知", "纯文本", "", "").
			WillReturnResult(sqlmock.NewResult(42, 1))
		mock.ExpectCommit()
		notice := HGNotice{Category: "system", Title: "通知", Content: "纯文本"}
		if err := service.HGDeliver(context.Background(), "evt-1", "U1", notice); err != nil {
			t.Fatal(err)
		}
		var event struct {
			EventType string `json:"event_type"` // 固定的收件箱变更提示。
			Category  string `json:"category"`   // 只传分类，不传用户或通知正文。
		}
		if json.Unmarshal(publisher.payload, &event) != nil || publisher.user != "U1" ||
			event.EventType != "notification.inbox.updated.v1" || event.Category != "system" || publisher.calls != 1 {
			t.Fatalf("网关故障=%t 时提示协议错误", gatewayFailure)
		}
		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(SQL.HGNotificationLedgerInsertSQL)).
			WithArgs(sqlmock.AnyArg(), "evt-1", "U1", "system").
			WillReturnError(&mysql.MySQLError{Number: 1062, Message: "duplicate key"})
		mock.ExpectRollback()
		if err := service.HGDeliver(context.Background(), "evt-1", "U1", notice); err != nil || publisher.calls != 1 {
			t.Fatalf("重复事件不得再次发布: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		db.Close()
	}
}

// 收件箱事务失败时不能发布实时消息，以免用户看到不存在的通知。
func TestHGNotificationHintNotPublishedOnRollback(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	hints, publisher := HGNewHints(), new(hgHintPublisher)
	hints.HGUsePublisher(publisher)
	service := HGNewService(HGNewRepository(db), hints)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(SQL.HGNotificationLedgerInsertSQL)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(regexp.QuoteMeta(SQL.HGNotificationCounterReadySQL)).
		WillReturnRows(sqlmock.NewRows([]string{"ready"}).AddRow(false))
	mock.ExpectExec(regexp.QuoteMeta(SQL.HGNotificationInsertSQL)).
		WillReturnError(errors.New("数据库写入失败"))
	mock.ExpectRollback()
	if err := service.HGDeliver(context.Background(), "evt-1", "U1",
		HGNotice{Category: "system", Title: "通知", Content: "纯文本"}); err == nil || publisher.calls != 0 {
		t.Fatalf("回滚后不应发送提示: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 旧版本收件箱已存在但账本未回填时只补墓碑，不制造新的实时提示。
func TestHGNotificationHintSkipsLegacyDuplicate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	hints, publisher := HGNewHints(), new(hgHintPublisher)
	hints.HGUsePublisher(publisher)
	service := HGNewService(HGNewRepository(db), hints)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(SQL.HGNotificationLedgerInsertSQL)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(regexp.QuoteMeta(SQL.HGNotificationCounterReadySQL)).
		WillReturnRows(sqlmock.NewRows([]string{"ready"}).AddRow(false))
	mock.ExpectExec(regexp.QuoteMeta(SQL.HGNotificationInsertSQL)).
		WillReturnError(&mysql.MySQLError{Number: 1062, Message: "duplicate key"})
	mock.ExpectCommit()
	if err := service.HGDeliver(context.Background(), "evt-1", "U1",
		HGNotice{Category: "system", Title: "通知", Content: "纯文本"}); err != nil || publisher.calls != 0 {
		t.Fatalf("老通知补账本不得再次提示: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 营销分类的用户偏好拒收只保留永久墓碑，不能向本人 Socket 声称有新通知。
func TestHGNotificationHintSkipsOptOut(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	hints, publisher := HGNewHints(), new(hgHintPublisher)
	hints.HGUsePublisher(publisher)
	service := HGNewService(HGNewRepository(db), hints)
	mock.ExpectQuery(regexp.QuoteMeta(SQL.HGNotificationPreferencesSQL)).WithArgs("U1").
		WillReturnRows(sqlmock.NewRows([]string{"preferences"}).AddRow([]byte(`{"categories":{"marketing":false},"channels":{"in_app":true},"digest":"instant"}`)))
	mock.ExpectExec(regexp.QuoteMeta(SQL.HGNotificationLedgerInsertSQL)).
		WithArgs(sqlmock.AnyArg(), "evt-2", "U1", "marketing").WillReturnResult(sqlmock.NewResult(1, 1))
	if err := service.HGDeliver(context.Background(), "evt-2", "U1",
		HGNotice{Category: "marketing", Title: "活动通知", Content: "纯文本"}); err != nil || publisher.calls != 0 {
		t.Fatalf("偏好拒收不能发 Socket 提示: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
