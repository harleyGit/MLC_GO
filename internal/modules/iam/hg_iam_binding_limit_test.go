package iam

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// 子集标记与权限、审计必须在同一授权事务中；空数组仍写标记，不能退化成完整角色权限。
func TestHGBindingPermissionLimitTx(t *testing.T) {
	service, mock := hgTestService(t)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMRolePermissions)).WithArgs(uint64(7), 201).
		WillReturnRows(sqlmock.NewRows([]string{"permission_code"}).AddRow("mall:product:create").AddRow("mall:product:update"))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMBindingIDForLimit)).WithArgs("TARGET", uint64(7), "shop", "shop-1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(18))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGInsertIAMBindingLimit)).WithArgs(uint64(18)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGDeleteIAMBindingAllowed)).WithArgs(uint64(18)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGInsertIAMBindingAllowed)).WithArgs(uint64(18), "mall:product:create").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGInsertIAMAudit)).WithArgs("ACTOR", "iam.binding.permissions_limit", "user", "TARGET", "request-1", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(19, 1))
	mock.ExpectCommit()
	tx, err := service.repo.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	codes := []string{"mall:product:create"}
	item := HGBatchGrantItem{RoleID: "7", ScopeType: "shop", ScopeID: "shop-1", Permissions: &codes}
	if err = service.repo.hgSetBindingPermissionLimitTx(context.Background(), tx, "ACTOR", "request-1", "TARGET", 7, item); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGBindingPermissionLimitRejectsOutsideRole(t *testing.T) {
	service, mock := hgTestService(t)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMRolePermissions)).WithArgs(uint64(7), 201).
		WillReturnRows(sqlmock.NewRows([]string{"permission_code"}).AddRow("mall:product:create"))
	mock.ExpectRollback()
	tx, err := service.repo.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	codes := []string{"iam:role:grant_direct"}
	item := HGBatchGrantItem{RoleID: "7", ScopeType: "global", Permissions: &codes}
	if err = service.repo.hgSetBindingPermissionLimitTx(context.Background(), tx, "ACTOR", "request-1", "TARGET", 7, item); !errors.Is(err, ErrHGInvalid) {
		t.Fatalf("不得提交角色未持有的权限: %v", err)
	}
	_ = tx.Rollback()
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 显式空子集不得被误判为 nil 的旧客户端协议；仍须保存标记并记审计。
func TestHGBindingPermissionLimitEmptySelection(t *testing.T) {
	service, mock := hgTestService(t)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMRolePermissions)).WithArgs(uint64(7), 201).
		WillReturnRows(sqlmock.NewRows([]string{"permission_code"}).AddRow("mall:product:create"))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMBindingIDForLimit)).WithArgs("TARGET", uint64(7), "global", "").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(18))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGInsertIAMBindingLimit)).WithArgs(uint64(18)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGDeleteIAMBindingAllowed)).WithArgs(uint64(18)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGInsertIAMAudit)).WithArgs("ACTOR", "iam.binding.permissions_limit", "user", "TARGET", "request-1", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(19, 1))
	mock.ExpectCommit()
	tx, err := service.repo.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	codes := []string{}
	item := HGBatchGrantItem{RoleID: "7", ScopeType: "global", Permissions: &codes}
	if err = service.repo.hgSetBindingPermissionLimitTx(context.Background(), tx, "ACTOR", "request-1", "TARGET", 7, item); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 权限子集写入失败应向事务调用方返回错误；不应提交标记或缓存版本。
func TestHGBindingPermissionLimitInsertFailureRollsBack(t *testing.T) {
	service, mock := hgTestService(t)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMRolePermissions)).WithArgs(uint64(7), 201).
		WillReturnRows(sqlmock.NewRows([]string{"permission_code"}).AddRow("mall:product:create"))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMBindingIDForLimit)).WithArgs("TARGET", uint64(7), "global", "").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(18))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGInsertIAMBindingLimit)).WithArgs(uint64(18)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGDeleteIAMBindingAllowed)).WithArgs(uint64(18)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGInsertIAMBindingAllowed)).WithArgs(uint64(18), "mall:product:create").
		WillReturnError(errors.New("database write failed"))
	mock.ExpectRollback()
	tx, err := service.repo.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	codes := []string{"mall:product:create"}
	item := HGBatchGrantItem{RoleID: "7", ScopeType: "global", Permissions: &codes}
	if err = service.repo.hgSetBindingPermissionLimitTx(context.Background(), tx, "ACTOR", "request-1", "TARGET", 7, item); err == nil {
		t.Fatal("权限写入失败时不能提交绑定")
	}
	_ = tx.Rollback()
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 绑定列表必须返回独立限制标记，避免管理端误把角色全部权限当作本用户已获权限。
func TestHGBindingListShowsPermissionLimit(t *testing.T) {
	service, mock := hgTestService(t)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMBindingsFirst)).WithArgs("TARGET", 3).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "role_id", "role_code", "scope_type", "scope_id", "expires_at", "permissions_limited",
		}).AddRow(18, "TARGET", 7, "reviewer", "global", "", nil, true).
			AddRow(17, "TARGET", 7, "reviewer", "shop", "shop-1", nil, false))
	bindings, more, err := service.repo.hgBindings(context.Background(), "TARGET", 0, 2)
	if err != nil || more || len(bindings) != 2 || !bindings[0].PermissionsLimited || bindings[1].PermissionsLimited {
		t.Fatalf("绑定限制标记错误: %v %v %#v", err, more, bindings)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 四条权威授权读取路径必须同样限制绑定权限；没有标记的旧绑定仍继承当前角色权限。
func TestHGBindingLimitAppliedToEveryPermissionQuery(t *testing.T) {
	for name, query := range map[string]string{
		"direct":   SQLQueriesPackage.HGSelectIAMPermission,
		"cache":    SQLQueriesPackage.HGSelectIAMStablePermission,
		"snapshot": SQLQueriesPackage.HGSelectIAMAccessGrants,
		"assets":   SQLQueriesPackage.HGSelectIAMAssetPermissions,
	} {
		if !strings.Contains(query, "lim.binding_id IS NULL OR allowp.binding_id IS NOT NULL") {
			t.Errorf("%s 未限制绑定权限子集", name)
		}
	}
}
