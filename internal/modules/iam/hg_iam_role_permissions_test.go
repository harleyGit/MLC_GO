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

func TestHGRequestRolePermissionsBuiltInPolicy(t *testing.T) {
	for _, tc := range []struct {
		name    string
		code    string
		builtIn bool
		allowed bool
	}{
		{"运维内置角色", "ops_engineer", true, true},
		{"超级管理员", "super_admin", true, false},
		{"其他内置角色", "security_admin", true, false},
		{"自定义角色", "custom_ops", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service, mock := hgTestService(t)
			mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMManager)).WithArgs("ACTOR").
				WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
			hgExpectPermission(mock, "ACTOR", "global", "", "iam:role:grant", true)
			hgExpectPermission(mock, "ACTOR", "global", "", "ops:metrics:read", true)
			mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMRoleVersion)).WithArgs(uint64(6)).
				WillReturnRows(sqlmock.NewRows([]string{"version", "built_in", "role_code"}).AddRow(4, tc.builtIn, tc.code))
			blocked := errors.New("模拟审批存储不可用")
			if tc.allowed {
				mock.ExpectBegin().WillReturnError(blocked)
			}
			_, err := service.RequestRolePermissions(context.Background(), "ACTOR", "req-1", "123456", 6, []string{"ops:metrics:read"})
			if tc.allowed && !errors.Is(err, blocked) || !tc.allowed && !errors.Is(err, ErrHGForbidden) {
				t.Fatalf("角色 %s 的权限申请结果错误: %v", tc.code, err)
			}
			if err = mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestHGRequestRolePermissionsRejectsUndelegablePermission(t *testing.T) {
	service, mock := hgTestService(t)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMManager)).WithArgs("ACTOR").
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	hgExpectPermission(mock, "ACTOR", "global", "", "iam:role:grant", true)
	hgExpectPermission(mock, "ACTOR", "global", "", "ops:metrics:read", false)
	_, err := service.RequestRolePermissions(context.Background(), "ACTOR", "req-1", "123456", 6, []string{"ops:metrics:read"})
	if !errors.Is(err, ErrHGForbidden) {
		t.Fatalf("不得申请自己没有的权限: %v", err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGSetRolePermissionsBuiltInPolicy(t *testing.T) {
	for _, tc := range []struct {
		name        string
		code        string
		builtIn     bool
		version     uint64
		expected    uint64
		updatedRows int64
		write       bool
		want        error
	}{
		{"运维内置角色审批成功", "ops_engineer", true, 4, 4, 1, true, nil},
		{"运维内置角色禁止跳过审批", "ops_engineer", true, 4, 0, 0, false, ErrHGForbidden},
		{"自定义角色保留原有写入", "custom_ops", false, 4, 0, 1, true, nil},
		{"超级管理员禁止修改", "super_admin", true, 4, 4, 0, false, ErrHGForbidden},
		{"其他内置角色禁止修改", "operator", true, 4, 4, 0, false, ErrHGForbidden},
		{"版本过期不覆盖", "ops_engineer", true, 5, 4, 0, false, ErrHGConflict},
		{"数据库条件不匹配时回滚", "ops_engineer", true, 4, 4, 0, true, ErrHGConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service, mock := hgTestService(t)
			mock.ExpectBegin()
			mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMRoleForPermissionWrite)).WithArgs(uint64(6)).
				WillReturnRows(sqlmock.NewRows([]string{"built_in", "version", "role_code"}).AddRow(tc.builtIn, tc.version, tc.code))
			if tc.write {
				mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGDeleteIAMRolePermissions)).WithArgs(uint64(6)).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGInsertIAMRolePermission)).WithArgs(uint64(6), "ops:metrics:read").
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGBumpIAMRoleVersion)).WithArgs(uint64(6)).
					WillReturnResult(sqlmock.NewResult(0, tc.updatedRows))
			}
			if tc.want == nil {
				mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGBumpIAMPolicyRevision)).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGInsertIAMAudit)).
					WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGInsertIAMDeliveryJob)).
					WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectCommit()
			} else {
				mock.ExpectRollback()
			}
			ctx := context.Background()
			tx, err := service.repo.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
			if err != nil {
				t.Fatal(err)
			}
			err = service.repo.hgSetRolePermissionsTx(ctx, tx, "APPROVER", "req-2", 6, []string{"ops:metrics:read"}, tc.expected)
			if err == nil {
				err = tx.Commit()
			} else {
				_ = tx.Rollback()
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("角色 %s 的权限更新结果错误: %v", tc.code, err)
			}
			if err = mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
