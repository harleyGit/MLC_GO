package repository

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestHGDeleteUnusedUser(t *testing.T) {
	for _, tc := range []struct {
		name         string
		wallet       *uint64
		frozen       uint64
		blocked      bool
		missing      bool
		auditFailure bool
	}{
		{name: "无关联无钱包"},
		{name: "清理零余额钱包", wallet: func() *uint64 { v := uint64(0); return &v }()},
		{name: "拒绝非零余额", wallet: func() *uint64 { v := uint64(10); return &v }(), blocked: true},
		{name: "拒绝冻结余额", wallet: func() *uint64 { v := uint64(0); return &v }(), frozen: 1, blocked: true},
		{name: "用户不存在", missing: true},
		{name: "审计失败回滚", auditFailure: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			mock.ExpectBegin()
			if tc.missing {
				mock.ExpectQuery(SQLQueriesPackage.HGUserDeleteLockSQL).WithArgs("user-1").WillReturnError(sql.ErrNoRows)
				mock.ExpectRollback()
			} else {
				mock.ExpectQuery(SQLQueriesPackage.HGUserDeleteLockSQL).WithArgs("user-1").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
				for _, query := range SQLQueriesPackage.HGUserDeleteDependencies {
					mock.ExpectQuery(query).WithArgs("user-1").WillReturnRows(sqlmock.NewRows([]string{"found"}))
				}
				for _, query := range SQLQueriesPackage.HGUserDeleteNumericDependencies {
					mock.ExpectQuery(query).WithArgs(1).WillReturnRows(sqlmock.NewRows([]string{"found"}))
				}
				if tc.wallet != nil {
					mock.ExpectQuery(SQLQueriesPackage.HGUserDeleteWalletSQL).WithArgs("user-1").WillReturnRows(sqlmock.NewRows([]string{"balance", "frozen_balance"}).AddRow(*tc.wallet, tc.frozen))
				} else {
					mock.ExpectQuery(SQLQueriesPackage.HGUserDeleteWalletSQL).WithArgs("user-1").WillReturnError(sql.ErrNoRows)
				}
				if tc.blocked {
					mock.ExpectRollback()
				} else {
					if tc.wallet != nil {
						mock.ExpectExec(SQLQueriesPackage.HGUserDeleteWalletRowSQL).WithArgs("user-1").WillReturnResult(sqlmock.NewResult(0, 1))
					}
					mock.ExpectExec(SQLQueriesPackage.HGUserDeleteAccountSQL).WithArgs("user-1").WillReturnResult(sqlmock.NewResult(0, 1))
					if tc.auditFailure {
						mock.ExpectExec(SQLQueriesPackage.HGUserDeleteAuditSQL).WithArgs("actor", "user-1").WillReturnError(errors.New("audit unavailable"))
						mock.ExpectRollback()
					} else {
						mock.ExpectExec(SQLQueriesPackage.HGUserDeleteAuditSQL).WithArgs("actor", "user-1").WillReturnResult(sqlmock.NewResult(1, 1))
						mock.ExpectCommit()
					}
				}
			}
			err = NewUserRepo(db).HGDeleteUnusedUser(context.Background(), "user-1", "actor")
			if tc.missing && !errors.Is(err, sql.ErrNoRows) || tc.blocked && !errors.Is(err, HGErrUserHasDependencies) || tc.auditFailure && err == nil || !tc.missing && !tc.blocked && !tc.auditFailure && err != nil {
				t.Fatalf("删除结果不符合预期: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestHGDeleteUnusedUserRejectsAssociationAndQueryFailure(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result error
	}{{"关联记录", nil}, {"查询失败", errors.New("database unavailable")}} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			mock.ExpectBegin()
			mock.ExpectQuery(SQLQueriesPackage.HGUserDeleteLockSQL).WithArgs("user-1").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
			if tc.result == nil {
				mock.ExpectQuery(SQLQueriesPackage.HGUserDeleteDependencies[0]).WithArgs("user-1").WillReturnRows(sqlmock.NewRows([]string{"found"}).AddRow(1))
			} else {
				mock.ExpectQuery(SQLQueriesPackage.HGUserDeleteDependencies[0]).WithArgs("user-1").WillReturnError(tc.result)
			}
			mock.ExpectRollback()
			err = NewUserRepo(db).HGDeleteUnusedUser(context.Background(), "user-1", "actor")
			if err == nil || tc.result == nil && !errors.Is(err, HGErrUserHasDependencies) {
				t.Fatalf("必须拒绝删除: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestHGCanonicalDeleteUserIDUsesStoredIdentity(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(SQLQueriesPackage.HGUserDeleteCanonicalIDSQL).WithArgs("user-a").WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow("User-A"))
	got, err := NewUserRepo(db).HGCanonicalDeleteUserID(context.Background(), "user-a")
	if err != nil || got != "User-A" {
		t.Fatalf("Redis 与数据库用户标识必须一致: %q %v", got, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
