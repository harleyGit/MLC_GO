package iam

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// 核心全局/店铺/旧组织类型不能由管理端停用，且不应打开事务或消费 MFA。
func TestHGScopeDisableRejectsCoreType(t *testing.T) {
	service, mock := hgTestService(t)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMManager)).WithArgs("ACTOR").WillReturnRows(sqlmock.NewRows([]string{"one"}).AddRow(1))
	hgExpectPermission(mock, "ACTOR", "global", "", "iam:role:grant", true)
	if err := service.HGDisableScopeType(context.Background(), "ACTOR", "request-1", "shop"); !errors.Is(err, ErrHGForbidden) {
		t.Fatalf("现有类型不可停用: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 停用时包括已撤销历史绑定都要拒绝；否则日后恢复绑定将指向失效类型。
func TestHGScopeDisableRejectsReferencedType(t *testing.T) {
	service, mock := hgTestService(t)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMManager)).WithArgs("ACTOR").WillReturnRows(sqlmock.NewRows([]string{"one"}).AddRow(1))
	hgExpectPermission(mock, "ACTOR", "global", "", "iam:role:grant", true)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMScopeTypeForUpdate)).WithArgs("region").WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMScopeBinding)).WithArgs("region").WillReturnRows(sqlmock.NewRows([]string{"one"}).AddRow(1))
	mock.ExpectRollback()
	if err := service.HGDisableScopeType(context.Background(), "ACTOR", "request-1", "region"); !errors.Is(err, ErrHGConflict) {
		t.Fatalf("引用中应拒绝停用: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 批量请求先验证组合个数和作用域 allowlist，拒绝未接入资源校验的字典类型。
func TestHGGrantDirectBatchRejectsUnverifiedScope(t *testing.T) {
	service, mock := hgTestService(t)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMManager)).WithArgs("ACTOR").WillReturnRows(sqlmock.NewRows([]string{"one"}).AddRow(1))
	hgExpectPermission(mock, "ACTOR", "global", "", "iam:role:grant", true)
	hgExpectPermission(mock, "ACTOR", "global", "", "iam:role:grant_direct", true)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMGrantSelf)).WithArgs("ACTOR", "TARGET").WillReturnRows(sqlmock.NewRows([]string{"one"}))
	err := service.HGGrantDirectBatch(context.Background(), "ACTOR", "request-1", HGBatchGrantRequest{UserID: "TARGET", Bindings: []HGBatchGrantItem{{RoleID: "7", ScopeType: "environment", ScopeID: "production"}}})
	if !errors.Is(err, ErrHGInvalid) {
		t.Fatalf("未校验资源的类型不得授予: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
