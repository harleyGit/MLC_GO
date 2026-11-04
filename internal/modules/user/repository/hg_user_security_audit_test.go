package repository

import (
	UserDtoPackage "MLC_GO/internal/modules/user/dto"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestUpdateSecurityByUserID_AdminAuditIsAtomic(t *testing.T) {
	for _, tc := range []struct {
		name      string
		auditFail bool
	}{
		{name: "审计成功时提交"},
		{name: "审计失败时回滚", auditFail: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			mock.ExpectBegin()
			mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectUserSecurityBaseForUpdateSQL)).WithArgs("target").
				WillReturnRows(sqlmock.NewRows([]string{"user_id", "email", "phone", "password_hash", "salt"}).AddRow("target", "old@example.test", "10000000000", "hash", "salt"))
			mock.ExpectExec("UPDATE users SET .* WHERE user_id = \\?").WithArgs("19900000000", "target").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectUserSecurityIDForUpdateSQL)).WithArgs("target").
				WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
			mock.ExpectExec("UPDATE user_security SET .* WHERE user_id = \\?").WithArgs("19900000000", "target").WillReturnResult(sqlmock.NewResult(0, 1))
			audit := mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGInsertUserDirectorySecurityAuditSQL)).WithArgs("admin", "target", "phone")
			if tc.auditFail {
				audit.WillReturnError(errors.New("audit unavailable"))
				mock.ExpectRollback()
			} else {
				audit.WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectCommit()
			}
			phone := "19900000000"
			err = NewUserRepo(db).UpdateSecurityByUserID(context.Background(), "target", &UserDtoPackage.HGUpdateUserSecurityReqDTO{Phone: &phone}, nil, nil, "admin")
			if (err != nil) != tc.auditFail {
				t.Fatalf("更新结果 err=%v, auditFail=%v", err, tc.auditFail)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
