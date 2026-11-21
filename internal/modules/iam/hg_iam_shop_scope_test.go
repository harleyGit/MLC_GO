package iam

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// 管理端选择已审核真实商家 ID 时，无须先手工登记 IAM 组织；不同作用域仍不能互相代替。
func TestHGTrustedScopeApprovedMerchantFallback(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMOrganization)).
		WithArgs("MER_APPROVED", "shop").WillReturnRows(sqlmock.NewRows([]string{"one"}))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMApprovedShopScope)).
		WithArgs("MER_APPROVED").WillReturnRows(sqlmock.NewRows([]string{"one"}).AddRow(1))
	allowed, err := NewHGRepository(db).hgOrganization(context.Background(), "shop", "MER_APPROVED")
	if err != nil || !allowed {
		t.Fatalf("已审核商家应可作为 shop 作用域: %v, %v", allowed, err)
	}
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMOrganization)).
		WithArgs("shop-registered", "shop").WillReturnRows(sqlmock.NewRows([]string{"one"}).AddRow(1))
	allowed, err = NewHGRepository(db).hgOrganization(context.Background(), "shop", "shop-registered")
	if err != nil || !allowed {
		t.Fatalf("已登记 shop 应沿用原有作用域: %v, %v", allowed, err)
	}
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMOrganization)).
		WithArgs("MER_PENDING", "shop").WillReturnRows(sqlmock.NewRows([]string{"one"}))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMApprovedShopScope)).
		WithArgs("MER_PENDING").WillReturnRows(sqlmock.NewRows([]string{"one"}))
	allowed, err = NewHGRepository(db).hgOrganization(context.Background(), "shop", "MER_PENDING")
	if err != nil || allowed {
		t.Fatalf("未审核商家不得授权: %v, %v", allowed, err)
	}
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMOrganization)).
		WithArgs("MER_APPROVED", "org").WillReturnRows(sqlmock.NewRows([]string{"one"}))
	allowed, err = NewHGRepository(db).hgOrganization(context.Background(), "org", "MER_APPROVED")
	if err != nil || allowed {
		t.Fatalf("真实商家 ID 不能作为 org 范围: %v, %v", allowed, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 事务重读商家审核状态；预检后若已撤销则拒绝且回滚，不声称锁定整个审核周期。
func TestHGGrantTxRejectsRevokedMerchantScope(t *testing.T) {
	service, mock := hgTestService(t)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMGrantTargetUser)).
		WithArgs("TARGET").WillReturnRows(sqlmock.NewRows([]string{"one"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMGrantSelf)).
		WithArgs("ACTOR", "TARGET").WillReturnRows(sqlmock.NewRows([]string{"one"}))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMOrganization)).
		WithArgs("MER_REVOKED", "shop").WillReturnRows(sqlmock.NewRows([]string{"one"}))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMApprovedShopScope)).
		WithArgs("MER_REVOKED").WillReturnRows(sqlmock.NewRows([]string{"one"}))
	mock.ExpectRollback()
	tx, err := service.repo.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	err = service.repo.hgGrantTx(context.Background(), tx, "ACTOR", "ACTOR", "request-1", "iam.binding.grant_direct", "TARGET", 7, "shop", "MER_REVOKED", nil)
	if !errors.Is(err, ErrHGScopeNotFound) {
		t.Fatalf("未审核商家在事务中必须拒绝: %v", err)
	}
	_ = tx.Rollback()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 即时授权事务同时确认目标账号、已审核商家、可委派权限及审计，不能仅预检通过。
func TestHGGrantTxApprovedMerchantWritesBinding(t *testing.T) {
	service, mock := hgTestService(t)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMGrantTargetUser)).
		WithArgs("TARGET").WillReturnRows(sqlmock.NewRows([]string{"one"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMGrantSelf)).
		WithArgs("ACTOR", "TARGET").WillReturnRows(sqlmock.NewRows([]string{"one"}))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMOrganization)).
		WithArgs("MER_APPROVED", "shop").WillReturnRows(sqlmock.NewRows([]string{"one"}))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMApprovedShopScope)).
		WithArgs("MER_APPROVED").WillReturnRows(sqlmock.NewRows([]string{"one"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMActiveRoleCode)).
		WithArgs(uint64(7)).WillReturnRows(sqlmock.NewRows([]string{"role_code"}).AddRow("operator"))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMRolePermissions)).
		WithArgs(uint64(7), 201).WillReturnRows(sqlmock.NewRows([]string{"permission_code"}).AddRow("mall:product:update"))
	hgExpectPermission(mock, "ACTOR", "global", "", "mall:product:update", true)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMBinding)).
		WithArgs("TARGET", uint64(7), "shop", "MER_APPROVED").WillReturnRows(sqlmock.NewRows([]string{"id", "status"}))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGInsertIAMBinding)).
		WithArgs("TARGET", uint64(7), "shop", "MER_APPROVED", nil, "ACTOR").WillReturnResult(sqlmock.NewResult(18, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGBumpIAMPolicyRevision)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGInsertIAMAudit)).
		WithArgs("ACTOR", "iam.binding.grant_direct", "user", "TARGET", "request-1", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(19, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGInsertIAMDeliveryJob)).
		WithArgs("user", "TARGET", "operator").WillReturnResult(sqlmock.NewResult(20, 1))
	mock.ExpectCommit()
	tx, err := service.repo.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if err = service.repo.hgGrantTx(context.Background(), tx, "ACTOR", "ACTOR", "request-1", "iam.binding.grant_direct", "TARGET", 7, "shop", "MER_APPROVED", nil); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 目标账号不存在与作用域不存在须区分，避免将用户 ID 问题误诊为商家问题。
func TestHGGrantTxRejectsMissingTargetUser(t *testing.T) {
	service, mock := hgTestService(t)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMGrantTargetUser)).
		WithArgs("MISSING").WillReturnRows(sqlmock.NewRows([]string{"one"}))
	mock.ExpectRollback()
	tx, err := service.repo.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	err = service.repo.hgGrantTx(context.Background(), tx, "ACTOR", "ACTOR", "request-1", "iam.binding.grant_direct", "MISSING", 7, "shop", "MER_APPROVED", nil)
	if !errors.Is(err, ErrHGNotFound) {
		t.Fatalf("目标用户不存在须继续返回目标不存在: %v", err)
	}
	_ = tx.Rollback()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// DB 错误不能被当作作用域不存在，否则会掩盖依赖故障。
func TestHGTrustedScopeDatabaseFailure(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	dbErr := errors.New("database unavailable")
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMOrganization)).
		WithArgs("MER_1", "shop").WillReturnError(dbErr)
	_, err = NewHGRepository(db).hgOrganization(context.Background(), "shop", "MER_1")
	if !errors.Is(err, dbErr) {
		t.Fatalf("数据库故障不能作为权限拒绝: %v", err)
	}
}

// HTTP 错误保留目标账号与商家作用域的不同提示，依赖故障仍是 500。
func TestHGShopScopeHTTPError(t *testing.T) {
	for _, tc := range []struct {
		err     error
		status  int
		message string
	}{
		{ErrHGScopeNotFound, http.StatusNotFound, "作用域不存在或未生效"},
		{ErrHGNotFound, http.StatusNotFound, "目标不存在"},
		{errors.New("database unavailable"), http.StatusInternalServerError, "IAM 服务暂不可用"},
	} {
		w := httptest.NewRecorder()
		hgHTTPError(w, httptest.NewRequest(http.MethodPost, "/api/v1/iam/bindings/grant-direct-batch", nil), tc.err)
		if w.Code != tc.status || !strings.Contains(w.Body.String(), tc.message) {
			t.Fatalf("%v 应返回 %d / %s，实际 %d / %s", tc.err, tc.status, tc.message, w.Code, w.Body.String())
		}
	}
}
