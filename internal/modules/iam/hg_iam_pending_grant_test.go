package iam

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

// TestHGListPendingGrants 验证目标用户、审批状态与游标的有界查询，并确保申请不被当作活动绑定。
func TestHGListPendingGrants(t *testing.T) {
	service, mock := hgTestService(t)
	hgManagerRead := func() {
		mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMManager)).WithArgs("ACTOR").WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
		hgExpectPermission(mock, "ACTOR", "global", "", "iam:role:read", true)
	}
	hgApprovalExpiry := time.Now().Add(time.Hour)
	hgCreated := time.Now().Add(-time.Minute)
	hgPayload := []byte(`{"userId":"TARGET","roleId":11,"scopeType":"global","scopeId":"","expiresAt":null}`)
	hgCols := []string{"id", "payload", "expires_at", "created_at"}
	hgManagerRead()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMPendingGrantsFirst)).WithArgs("TARGET", 2).
		WillReturnRows(sqlmock.NewRows(hgCols).AddRow(5, hgPayload, hgApprovalExpiry, hgCreated).AddRow(4, hgPayload, hgApprovalExpiry, hgCreated)).RowsWillBeClosed()
	items, next, more, err := service.ListPendingGrants(context.Background(), "ACTOR", " TARGET ", 0, 1)
	if err != nil || !more || next != "5" || len(items) != 1 || items[0].RoleID != "11" || items[0].ApprovalID != "5" || items[0].Status != "pending" {
		t.Fatalf("第一页结果: items=%+v next=%q more=%t err=%v", items, next, more, err)
	}
	hgManagerRead()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMPendingGrantsAfter)).WithArgs("TARGET", uint64(5), 2).
		WillReturnRows(sqlmock.NewRows(hgCols).AddRow(4, hgPayload, hgApprovalExpiry, hgCreated)).RowsWillBeClosed()
	items, next, more, err = service.ListPendingGrants(context.Background(), "ACTOR", "TARGET", 5, 1)
	if err != nil || more || next != "4" || len(items) != 1 {
		t.Fatalf("末页结果: items=%+v next=%q more=%t err=%v", items, next, more, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// TestHGListPendingGrantsRejectsUnauthorizedAndMismatchedPayload 拒绝越权读取与数据错配，不能泄露其他管理员审批内容。
func TestHGListPendingGrantsRejectsUnauthorizedAndMismatchedPayload(t *testing.T) {
	service, mock := hgTestService(t)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMManager)).WithArgs("ACTOR").WillReturnRows(sqlmock.NewRows([]string{"1"}))
	if _, _, _, err := service.ListPendingGrants(context.Background(), "ACTOR", "TARGET", 0, 20); !errors.Is(err, ErrHGForbidden) {
		t.Fatalf("无权限读请求未拒绝: %v", err)
	}
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMManager)).WithArgs("ACTOR").WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	hgExpectPermission(mock, "ACTOR", "global", "", "iam:role:read", true)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMPendingGrantsFirst)).WithArgs("TARGET", 21).
		WillReturnRows(sqlmock.NewRows([]string{"id", "payload", "expires_at", "created_at"}).AddRow(5, []byte(`{"userId":"OTHER","roleId":11}`), time.Now().Add(time.Hour), time.Now())).RowsWillBeClosed()
	if _, _, _, err := service.ListPendingGrants(context.Background(), "ACTOR", "TARGET", 0, 20); !errors.Is(err, ErrHGInvalid) {
		t.Fatalf("目标用户错配未拒绝: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
