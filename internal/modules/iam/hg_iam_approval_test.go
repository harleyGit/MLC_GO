package iam

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

// 策略版本变化后使用全新 Redis key，旧授权缓存不会被再次命中。
func TestHGPermissionCacheKeyIsVersionedAndOpaque(t *testing.T) {
	before := PersistenceRedisPackage.GetIAMPermissionKey(7, "PRIVATE_USER", "asset:coin:grant", "global", "")
	after := PersistenceRedisPackage.GetIAMPermissionKey(8, "PRIVATE_USER", "asset:coin:grant", "global", "")
	if before == after || strings.Contains(before, "PRIVATE_USER") {
		t.Fatalf("策略版本失效或 key 泄露业务 ID: %s", before)
	}
}

// 撤权必须在提交前递增权威策略版本；版本更新失败时整笔撤权回滚。
func TestHGRevokeRollsBackOnRevisionFailure(t *testing.T) {
	service, mock := hgTestService(t)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMBindingRoleForRevoke)).WithArgs(uint64(8), "USER").WillReturnRows(sqlmock.NewRows([]string{"role_code"}).AddRow("operator"))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGRevokeIAMBinding)).WithArgs(uint64(8), "USER").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGBumpIAMPolicyRevision)).WillReturnError(errors.New("revision unavailable"))
	mock.ExpectRollback()
	err := service.repo.hgRevoke(context.Background(), "ADMIN", "req-1", "USER", 8)
	if err == nil || err.Error() != "revision unavailable" {
		t.Fatalf("版本失败未回滚: %v", err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 普通管理员即使拥有管理和审批权限，也不能批准本人申请。
func TestHGApprovalRejectsSelfDecision(t *testing.T) {
	service, mock := hgTestService(t)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMManager)).WithArgs("ADMIN").WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	hgExpectPermission(mock, "ADMIN", "global", "", "iam:role:grant", true)
	hgExpectPermission(mock, "ADMIN", "global", "", "iam:approval:approve", true)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMApprovalForUpdate)).WithArgs(uint64(9)).WillReturnRows(sqlmock.NewRows([]string{"applicant_id", "action", "payload", "status", "expires_at"}).AddRow("ADMIN", "grant", []byte(`{}`), "pending", time.Now().Add(time.Hour)))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMGlobalSuperAdminForApproval)).WithArgs("ADMIN").WillReturnRows(sqlmock.NewRows([]string{"1"}))
	mock.ExpectRollback()
	err := service.DecideApproval(context.Background(), "ADMIN", "req-1", "123456", 9, true)
	if !errors.Is(err, ErrHGConflict) {
		t.Fatalf("自批未被拒绝: %v", err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 有效全局超管绑定允许本人走审批事务，但仍须校验管理员权限及本人 MFA，不能直接生效。
func TestHGApprovalAllowsGlobalSuperAdminSelfDecisionBeforeMFA(t *testing.T) {
	service, mock := hgTestService(t)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMManager)).WithArgs("ADMIN").WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	hgExpectPermission(mock, "ADMIN", "global", "", "iam:role:grant", true)
	hgExpectPermission(mock, "ADMIN", "global", "", "iam:approval:approve", true)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMApprovalForUpdate)).WithArgs(uint64(9)).WillReturnRows(sqlmock.NewRows([]string{"applicant_id", "action", "payload", "status", "expires_at"}).AddRow("ADMIN", "grant", []byte(`{}`), "pending", time.Now().Add(time.Hour)))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMGlobalSuperAdminForApproval)).WithArgs("ADMIN").WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMManager)).WithArgs("ADMIN").WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	hgExpectPermission(mock, "ADMIN", "global", "", "iam:approval:approve", true)
	mock.ExpectRollback()
	err := service.DecideApproval(context.Background(), "ADMIN", "req-1", "123456", 9, true)
	if !errors.Is(err, ErrHGMFAUnavailable) {
		t.Fatalf("超管自审应继续进入本人 MFA 校验，实际 %v", err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 全局超管绑定查询故障时不能回退为自审放行，错误仍向调用方传递。
func TestHGApprovalRejectsSelfDecisionOnSuperAdminCheckError(t *testing.T) {
	service, mock := hgTestService(t)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMManager)).WithArgs("ADMIN").WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	hgExpectPermission(mock, "ADMIN", "global", "", "iam:role:grant", true)
	hgExpectPermission(mock, "ADMIN", "global", "", "iam:approval:approve", true)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMApprovalForUpdate)).WithArgs(uint64(9)).WillReturnRows(sqlmock.NewRows([]string{"applicant_id", "action", "payload", "status", "expires_at"}).AddRow("ADMIN", "role_permissions", []byte(`{}`), "pending", time.Now().Add(time.Hour)))
	checkErr := errors.New("super admin lookup unavailable")
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMGlobalSuperAdminForApproval)).WithArgs("ADMIN").WillReturnError(checkErr)
	mock.ExpectRollback()
	if err := service.DecideApproval(context.Background(), "ADMIN", "req-1", "123456", 9, true); !errors.Is(err, checkErr) {
		t.Fatalf("超管校验故障不得放行，实际 %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 另一名管理员仍按原审批链办理，不需检查超级管理员角色。
func TestHGApprovalKeepsOtherAdminDecisionBeforeMFA(t *testing.T) {
	service, mock := hgTestService(t)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMManager)).WithArgs("APPROVER").WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	hgExpectPermission(mock, "APPROVER", "global", "", "iam:role:grant", true)
	hgExpectPermission(mock, "APPROVER", "global", "", "iam:approval:approve", true)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMApprovalForUpdate)).WithArgs(uint64(9)).WillReturnRows(sqlmock.NewRows([]string{"applicant_id", "action", "payload", "status", "expires_at"}).AddRow("APPLICANT", "grant", []byte(`{}`), "pending", time.Now().Add(time.Hour)))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMManager)).WithArgs("APPROVER").WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	hgExpectPermission(mock, "APPROVER", "global", "", "iam:approval:approve", true)
	mock.ExpectRollback()
	if err := service.DecideApproval(context.Background(), "APPROVER", "req-1", "123456", 9, true); !errors.Is(err, ErrHGMFAUnavailable) {
		t.Fatalf("另一名管理员应继续进入 MFA 校验，实际 %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// /bindings/grant 的目标来自请求体，申请人已有全局权限时仍须要求目标为活动管理员。
func TestHGRequestGrantRejectsInactiveTargetBeforeMFA(t *testing.T) {
	service, mock := hgTestService(t)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMManager)).WithArgs("ACTOR").
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	hgExpectPermission(mock, "ACTOR", "global", "", "iam:role:grant", true)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMManager)).WithArgs("TARGET").
		WillReturnRows(sqlmock.NewRows([]string{"1"}))
	_, err := service.RequestGrant(context.Background(), "ACTOR", "req-1", "invalid", hgGrantApprovalPayload{
		UserID: "TARGET", RoleID: 8, ScopeType: "global",
	})
	if !errors.Is(err, ErrHGGrantTarget) {
		t.Fatalf("目标非活动管理员应返回专用错误，实际 %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
