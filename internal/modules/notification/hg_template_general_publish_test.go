package notification

import (
	queries "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// hgGeneralPublishRow 提供已保存的当前版本，不借助真实数据库或投递服务。
func hgGeneralPublishRow() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "template_code", "name", "category", "status", "version", "published_version",
		"title", "body", "schema", "action", "config", "channels", "priority", "locale", "publication_status",
	}).AddRow(7, "security.code", "平台验证码", "security", "draft", 2, nil,
		"验证提醒", "请查看 {{code}}", []byte(`{"code":{"type":"string"}}`), "none", []byte(`{}`),
		[]byte(`["in_app","realtime"]`), 2, "zh-CN", "draft")
}

func TestHGGeneralPublishSelfSuperAdminOnlyPublishesVersion(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationTemplateGeneralPublishLockSQL)).
		WithArgs("ADMIN", uint64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"latest", "status", "code", "self_review"}).
			AddRow(2, "draft", "security.code", true))
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendSelfPublishSuperAdminSQL)).
		WithArgs("ADMIN").WillReturnRows(sqlmock.NewRows([]string{"allowed"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationTemplateDetailSQL)).
		WithArgs(uint64(7)).WillReturnRows(hgGeneralPublishRow())
	mock.ExpectExec(regexp.QuoteMeta(queries.HGNotificationTemplateGeneralVersionUpdateSQL)).
		WithArgs(uint64(7), uint32(2)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(queries.HGNotificationTemplatePublishSQL)).
		WithArgs(uint32(2), "ADMIN", uint64(7), uint32(2)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(queries.HGNotificationTemplateAuditSQL)).
		WithArgs(uint64(7), uint32(2), "published_general_self", "ADMIN", "trace-1", "127.0.0.1", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := HGNewTemplateRepository(db).HGPublishGeneral(context.Background(), 7, 2, "ADMIN",
		HGAuditContext{RequestID: "trace-1", SourceIP: "127.0.0.1"}); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGGeneralPublishRejectsSelfWithoutRoleAndStaleVersion(t *testing.T) {
	for _, hgCase := range []struct {
		name    string
		version uint32
		lookup  bool
	}{
		{name: "未获超级管理员授权", version: 2, lookup: true},
		{name: "过期版本", version: 1},
	} {
		t.Run(hgCase.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			mock.ExpectBegin()
			mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationTemplateGeneralPublishLockSQL)).
				WithArgs("ADMIN", uint64(7)).
				WillReturnRows(sqlmock.NewRows([]string{"latest", "status", "code", "self_review"}).
					AddRow(2, "draft", "security.code", true))
			if hgCase.lookup {
				mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendSelfPublishSuperAdminSQL)).
					WithArgs("ADMIN").WillReturnRows(sqlmock.NewRows([]string{"allowed"}))
			}
			mock.ExpectRollback()
			if err := HGNewTemplateRepository(db).HGPublishGeneral(context.Background(), 7, hgCase.version, "ADMIN", HGAuditContext{}); !errors.Is(err, HGErrTemplateConflict) {
				t.Fatalf("不得发布: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestHGGeneralPublishRejectsRoleTemplate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationTemplateGeneralPublishLockSQL)).
		WithArgs("ADMIN", uint64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"latest", "status", "code", "self_review"}).
			AddRow(2, "draft", "iam.role.updated", true))
	mock.ExpectRollback()
	if err := HGNewTemplateRepository(db).HGPublishGeneral(context.Background(), 7, 2, "ADMIN", HGAuditContext{}); !errors.Is(err, HGErrTemplateConflict) {
		t.Fatalf("角色更新模板不得绕过专用发布: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGGeneralPublishAuditFailureRollsBack(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationTemplateGeneralPublishLockSQL)).
		WithArgs("REVIEWER", uint64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"latest", "status", "code", "self_review"}).
			AddRow(2, "draft", "security.code", false))
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationTemplateDetailSQL)).
		WithArgs(uint64(7)).WillReturnRows(hgGeneralPublishRow())
	mock.ExpectExec(regexp.QuoteMeta(queries.HGNotificationTemplateGeneralVersionUpdateSQL)).
		WithArgs(uint64(7), uint32(2)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(queries.HGNotificationTemplatePublishSQL)).
		WithArgs(uint32(2), "REVIEWER", uint64(7), uint32(2)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(queries.HGNotificationTemplateAuditSQL)).
		WithArgs(uint64(7), uint32(2), "published_general", "REVIEWER", "", "", sqlmock.AnyArg()).
		WillReturnError(sql.ErrConnDone)
	mock.ExpectRollback()
	if err := HGNewTemplateRepository(db).HGPublishGeneral(context.Background(), 7, 2, "REVIEWER", HGAuditContext{}); err == nil {
		t.Fatal("审计写入失败必须回滚通用发布")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
