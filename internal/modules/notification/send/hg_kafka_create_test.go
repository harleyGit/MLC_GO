package send

import (
	queries "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestHGNewKafkaRepositoryCreatesOnlyKafkaModeTasks(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cmd := HGCommand{
		TemplateID: "7", TemplateVersion: 2, UserIDs: []string{"user_1"},
		Variables: map[string]string{"name": "用户A", "when": "02:00"},
	}
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendByRequestSQL)).WithArgs("kafka-1").WillReturnError(sql.ErrNoRows)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendTemplateSQL)).WithArgs(uint32(2), uint64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"category", "title", "body", "schema", "action", "config", "channels"}).
			AddRow("system", "维护 {{name}}", "您好 {{name}}，维护将在 {{when}} 开始。",
				[]byte(hgVersion().Schema), "none", []byte(`{}`), []byte(`["in_app"]`)))
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendUsersPrefixSQL)).
		WithArgs("user_1").WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow("user_1"))
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendAdminLockSQL)).WithArgs("ADMIN").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendPendingLimitSQL)).WithArgs("ADMIN").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec(regexp.QuoteMeta(queries.HGNotificationKafkaInsertSQL)).
		WithArgs("kafka-1", sqlmock.AnyArg(), uint64(7), uint32(2),
			[]byte(`["user_1"]`), sqlmock.AnyArg(), "ADMIN").
		WillReturnResult(sqlmock.NewResult(78, 1))
	mock.ExpectExec(regexp.QuoteMeta(queries.HGNotificationSendAuditSQL)).
		WithArgs(int64(78), "created", "ADMIN", uint32(2), "", "").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	if id, err := HGNewKafkaRepository(db).HGCreate(context.Background(), "kafka-1", "ADMIN", cmd); err != nil || id != "78" {
		t.Fatalf("仅 Kafka 模式创建新任务: %q %v", id, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
