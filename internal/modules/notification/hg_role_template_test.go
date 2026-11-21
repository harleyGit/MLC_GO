package notification

import (
	queries "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestHGRolePublishRequiresRealtimeAndDeclaredRole(t *testing.T) {
	title, body := "角色信息已更新", "您的角色 {{role_name}} 已更新。"
	schema := []byte(`{"role_name":{"type":"string","required":true,"max_length":128}}`)
	if err := HGValidateRolePublished(title, body, schema, []byte(`["in_app","realtime"]`)); err != nil {
		t.Fatal(err)
	}
	for _, channels := range [][]byte{[]byte(`["in_app"]`), []byte(`["realtime"]`), []byte(`[]`)} {
		if !errors.Is(HGValidateRolePublished(title, body, schema, channels), HGErrInvalid) {
			t.Fatalf("没有双渠道的模板不能发布: %s", channels)
		}
	}
}

func TestHGRolePublishPinsStoredVersion(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := HGNewTemplateRepository(db)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationTemplatePublishLockSQL)).WithArgs(uint64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"version", "status", "template_code"}).AddRow(3, "draft", "iam.role.updated"))
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationTemplatePublishVersionSQL)).WithArgs(uint64(7), uint32(3)).
		WillReturnRows(sqlmock.NewRows([]string{"title", "body", "schema", "channels"}).
			AddRow("角色信息已更新", "您的角色 {{role_name}} 已更新。",
				[]byte(`{"role_name":{"type":"string","required":true,"max_length":128}}`), []byte(`["in_app","realtime"]`)))
	mock.ExpectExec(regexp.QuoteMeta(queries.HGNotificationTemplatePublishVersionUpdateSQL)).
		WithArgs(uint64(7), uint32(3)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(queries.HGNotificationTemplatePublishSQL)).
		WithArgs(uint32(3), "ADMIN", uint64(7), uint32(3)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := repo.HGPublish(context.Background(), 7, 3, "ADMIN"); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
