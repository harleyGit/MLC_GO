package iam

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func hgTestService(t *testing.T) (*HGService, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewHGService(NewHGRepository(db)), mock
}

func hgExpectPermission(mock sqlmock.Sqlmock, userID, scopeType, scopeID, permission string, allowed bool) {
	rows := sqlmock.NewRows([]string{"1"})
	if allowed {
		rows.AddRow(1)
	}
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMPermission)).
		WithArgs(userID, scopeType, scopeID, permission).WillReturnRows(rows)
}

// 跨店铺授权必须拒绝，且不能把某店铺的角色当成全局角色。
func TestHGAuthorizeRejectsOtherScope(t *testing.T) {
	service, mock := hgTestService(t)
	hgExpectPermission(mock, "U1", "shop", "SHOP_B", "mall:product:update", false)
	hgExpectPermission(mock, "U1", "global", "", "mall:product:update", false)
	decision, err := service.Authorize(context.Background(), "U1", "mall:product:update", HGResource{ScopeType: "shop", ScopeID: "SHOP_B"})
	if err != nil || decision.Allowed {
		t.Fatalf("跨店铺授权结果错误: decision=%+v err=%v", decision, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 资源归属和状态在数据库权限查询前判定，避免有角色却越权修改其他人的资源。
func TestHGAuthorizeRejectsOwnerAndStatus(t *testing.T) {
	service, mock := hgTestService(t)
	for _, resource := range []HGResource{
		{OwnerID: "U2", RequireOwner: true},
		{OwnerID: "U1", RequireOwner: true, Status: "published", AllowedStatuses: []string{"draft"}},
	} {
		decision, err := service.Authorize(context.Background(), "U1", "video:submission:update", resource)
		if err != nil || decision.Allowed {
			t.Fatalf("资源属性越权未拒绝: decision=%+v err=%v", decision, err)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 只有 iam:role:grant 的安全管理员不能授予自己尚未拥有的业务权限。
func TestHGGrantRejectsPrivilegeEscalation(t *testing.T) {
	service, mock := hgTestService(t)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMManager)).WithArgs("ACTOR").WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	hgExpectPermission(mock, "ACTOR", "global", "", "iam:role:grant", true)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMManager)).WithArgs("TARGET").WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMRolePermissions)).WithArgs(uint64(7), 201).WillReturnRows(sqlmock.NewRows([]string{"permission_code"}).AddRow("ops:metrics:read"))
	hgExpectPermission(mock, "ACTOR", "global", "", "ops:metrics:read", false)
	err := service.Grant(context.Background(), "ACTOR", "request-1", "TARGET", 7, "global", "", nil)
	if !errors.Is(err, ErrHGForbidden) {
		t.Fatalf("预期拒绝越权授权，实际为 %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 管理员账号停用时，角色绑定不能继续提供管理端读权限。
func TestHGListRolesRejectsInactiveAdmin(t *testing.T) {
	service, mock := hgTestService(t)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMManager)).WithArgs("ACTOR").WillReturnError(sql.ErrNoRows)
	_, _, _, err := service.ListRoles(context.Background(), "ACTOR", 0, 20)
	if !errors.Is(err, ErrHGForbidden) {
		t.Fatalf("预期拒绝停用管理员，实际为 %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 审计写入失败必须回滚角色创建，避免产生无法追溯的授权对象。
func TestHGCreateRoleRollsBackWhenAuditFails(t *testing.T) {
	service, mock := hgTestService(t)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGInsertIAMRole)).WithArgs("reviewer", "审核员", "").WillReturnResult(sqlmock.NewResult(7, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGInsertIAMAudit)).WillReturnError(errors.New("audit unavailable"))
	mock.ExpectRollback()
	_, err := service.repo.hgCreateRole(context.Background(), "ACTOR", "request-1", "reviewer", "审核员", "", nil)
	if err == nil {
		t.Fatal("审计失败后创建角色应失败")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
