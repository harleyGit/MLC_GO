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

// 即使审批人拥有管理和审批权限，也不能批准本人申请。
func TestHGApprovalRejectsSelfDecision(t *testing.T) {
	service, mock := hgTestService(t)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMManager)).WithArgs("ADMIN").WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	hgExpectPermission(mock, "ADMIN", "global", "", "iam:role:grant", true)
	hgExpectPermission(mock, "ADMIN", "global", "", "iam:approval:approve", true)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMApprovalForUpdate)).WithArgs(uint64(9)).WillReturnRows(sqlmock.NewRows([]string{"applicant_id", "action", "payload", "status", "expires_at"}).AddRow("ADMIN", "grant", []byte(`{}`), "pending", time.Now().Add(time.Hour)))
	mock.ExpectRollback()
	err := service.DecideApproval(context.Background(), "ADMIN", "req-1", "123456", 9, true)
	if !errors.Is(err, ErrHGConflict) {
		t.Fatalf("自批未被拒绝: %v", err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
