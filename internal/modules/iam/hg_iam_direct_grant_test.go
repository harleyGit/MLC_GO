package iam

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// 本人不能绕过审批给自己赋权；拒绝须发生在任何 SQL 或 MFA 请求之前。
func TestHGGrantDirectRejectsSelf(t *testing.T) {
	service, mock := hgTestService(t)
	if err := service.HGGrantDirect(context.Background(), "ACTOR", "request-1", "123456", " ACTOR ", 7, "global", "", nil); !errors.Is(err, ErrHGForbidden) {
		t.Fatalf("即时自授应拒绝: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 普通申请权限不能替代即时授权专用权限，拒绝时不消费 MFA 或写入角色绑定。
func TestHGGrantDirectRejectsWithoutDedicatedPermission(t *testing.T) {
	service, mock := hgTestService(t)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMManager)).WithArgs("ACTOR").WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	hgExpectPermission(mock, "ACTOR", "global", "", "iam:role:grant", true)
	hgExpectPermission(mock, "ACTOR", "global", "", "iam:role:grant_direct", false)
	if err := service.HGGrantDirect(context.Background(), "ACTOR", "request-1", "123456", "TARGET", 7, "global", "", nil); !errors.Is(err, ErrHGForbidden) {
		t.Fatalf("缺少即时授权权限应拒绝: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 数据库唯一键排序规则可能把不同大小写的 user_id 视为同一账号；必须在消耗 MFA 前拒绝。
func TestHGGrantDirectRejectsSelfAlias(t *testing.T) {
	service, mock := hgTestService(t)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMManager)).WithArgs("ACTOR").WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	hgExpectPermission(mock, "ACTOR", "global", "", "iam:role:grant", true)
	hgExpectPermission(mock, "ACTOR", "global", "", "iam:role:grant_direct", true)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMManager)).WithArgs("ACTOR").WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	hgExpectPermission(mock, "ACTOR", "global", "", "iam:role:grant", true)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMGrantTargetUser)).WithArgs("actor").WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMRolePermissions)).WithArgs(uint64(7), 201).WillReturnRows(sqlmock.NewRows([]string{"permission_code"}))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMGrantSelf)).WithArgs("ACTOR", "actor").WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	if err := service.HGGrantDirect(context.Background(), "ACTOR", "request-1", "123456", "actor", 7, "global", "", nil); !errors.Is(err, ErrHGForbidden) {
		t.Fatalf("大小写别名自授应拒绝: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 持有专用权限仍须校验对目标角色的全部委派权限，不允许提升自己没有的业务能力。
func TestHGGrantDirectRejectsUndelegableRole(t *testing.T) {
	service, mock := hgTestService(t)
	for i := 0; i < 2; i++ {
		mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMManager)).WithArgs("ACTOR").WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
		hgExpectPermission(mock, "ACTOR", "global", "", "iam:role:grant", true)
		if i == 0 {
			hgExpectPermission(mock, "ACTOR", "global", "", "iam:role:grant_direct", true)
		}
	}
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMGrantTargetUser)).WithArgs("TARGET").WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMRolePermissions)).WithArgs(uint64(7), 201).WillReturnRows(sqlmock.NewRows([]string{"permission_code"}).AddRow("mall:product:create"))
	hgExpectPermission(mock, "ACTOR", "global", "", "mall:product:create", false)
	if err := service.HGGrantDirect(context.Background(), "ACTOR", "request-1", "123456", "TARGET", 7, "global", "", nil); !errors.Is(err, ErrHGForbidden) {
		t.Fatalf("无法委派的角色应拒绝: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 事务内禁止直接授予 super_admin，保持既有审批路径，并确保不会写绑定或审计。
func TestHGGrantDirectRejectsSuperAdminInTransaction(t *testing.T) {
	service, mock := hgTestService(t)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMGrantTargetUser)).WithArgs("TARGET").WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMGrantSelf)).WithArgs("ACTOR", "TARGET").WillReturnRows(sqlmock.NewRows([]string{"1"}))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMActiveRoleCode)).WithArgs(uint64(7)).WillReturnRows(sqlmock.NewRows([]string{"role_code"}).AddRow("super_admin"))
	mock.ExpectRollback()
	tx, err := service.repo.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err = service.repo.hgGrantTx(context.Background(), tx, "ACTOR", "ACTOR", "request-1", "iam.binding.grant_direct", "TARGET", 7, "global", "", nil); !errors.Is(err, ErrHGForbidden) {
		t.Fatalf("超级管理员角色不能直接授予: %v", err)
	}
	_ = tx.Rollback()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 非超级管理员角色可在同一事务中完成角色锁、可委派权限复核、绑定写入、缓存版本推进及专用审计。
func TestHGGrantDirectTransactionWritesBindingAndAudit(t *testing.T) {
	service, mock := hgTestService(t)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMGrantTargetUser)).WithArgs("TARGET").WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMGrantSelf)).WithArgs("ACTOR", "TARGET").WillReturnRows(sqlmock.NewRows([]string{"1"}))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMOrganization)).WithArgs("shop-1", "shop").WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMActiveRoleCode)).WithArgs(uint64(7)).WillReturnRows(sqlmock.NewRows([]string{"role_code"}).AddRow("reviewer"))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMRolePermissions)).WithArgs(uint64(7), 201).WillReturnRows(sqlmock.NewRows([]string{"permission_code"}).AddRow("mall:product:create"))
	hgExpectPermission(mock, "ACTOR", "global", "", "mall:product:create", true)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMBinding)).WithArgs("TARGET", uint64(7), "shop", "shop-1").WillReturnRows(sqlmock.NewRows([]string{"id", "status"}))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGInsertIAMBinding)).WithArgs("TARGET", uint64(7), "shop", "shop-1", nil, "ACTOR").WillReturnResult(sqlmock.NewResult(18, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGBumpIAMPolicyRevision)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGInsertIAMAudit)).WithArgs("ACTOR", "iam.binding.grant_direct", "user", "TARGET", "request-1", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(19, 1))
	mock.ExpectCommit()
	tx, err := service.repo.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if err = service.repo.hgGrantTx(context.Background(), tx, "ACTOR", "ACTOR", "request-1", "iam.binding.grant_direct", "TARGET", 7, "shop", "shop-1", nil); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 撤销后由旧客户端重新授权必须清除历史绑定的权限子集，否则旧协议无法恢复默认完整角色权限。
func TestHGGrantDirectReactivationClearsOldBindingLimit(t *testing.T) {
	service, mock := hgTestService(t)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMGrantTargetUser)).WithArgs("TARGET").WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMGrantSelf)).WithArgs("ACTOR", "TARGET").WillReturnRows(sqlmock.NewRows([]string{"1"}))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMActiveRoleCode)).WithArgs(uint64(7)).WillReturnRows(sqlmock.NewRows([]string{"role_code"}).AddRow("reviewer"))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMRolePermissions)).WithArgs(uint64(7), 201).WillReturnRows(sqlmock.NewRows([]string{"permission_code"}).AddRow("mall:product:create"))
	hgExpectPermission(mock, "ACTOR", "global", "", "mall:product:create", true)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMBinding)).WithArgs("TARGET", uint64(7), "global", "").
		WillReturnRows(sqlmock.NewRows([]string{"id", "status"}).AddRow(18, 0))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGReactivateIAMBinding)).WithArgs(nil, "ACTOR", uint64(18)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGDeleteIAMBindingLimit)).WithArgs(uint64(18)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGBumpIAMPolicyRevision)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGInsertIAMAudit)).WithArgs("ACTOR", "iam.binding.grant_direct", "user", "TARGET", "request-1", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(19, 1))
	mock.ExpectCommit()
	tx, err := service.repo.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if err = service.repo.hgGrantTx(context.Background(), tx, "ACTOR", "ACTOR", "request-1", "iam.binding.grant_direct", "TARGET", 7, "global", "", nil); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
