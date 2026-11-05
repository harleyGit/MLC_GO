package repository

import (
	UserDtoPackage "MLC_GO/internal/modules/user/dto"
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestHGUserRepoUpdateProfileZeroAffected(t *testing.T) {
	for _, tc := range []struct {
		name        string
		rows        int64
		exists      bool
		lookupError error
		wantError   error
	}{
		{name: "资料未变化但用户存在", exists: true},
		{name: "目标用户不存在", wantError: sql.ErrNoRows},
		{name: "回查失败", lookupError: context.DeadlineExceeded, wantError: context.DeadlineExceeded},
		{name: "资料已变化", rows: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()

			mock.ExpectExec(regexp.QuoteMeta("UPDATE users SET `nickname` = ? WHERE user_id = ?")).
				WithArgs("新昵称", "target").WillReturnResult(sqlmock.NewResult(0, tc.rows))
			if tc.rows == 0 {
				lookup := mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM users WHERE user_id = ? LIMIT 1")).WithArgs("target")
				switch {
				case tc.lookupError != nil:
					lookup.WillReturnError(tc.lookupError)
				case tc.exists:
					lookup.WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(33))
				default:
					lookup.WillReturnRows(sqlmock.NewRows([]string{"id"}))
				}
			}
			nickname := "新昵称"
			err = NewUserRepo(db).UpdateProfileByUserID(context.Background(), "target", &UserDtoPackage.HGUpdateUserProfileReqDTO{Nickname: &nickname})
			if (tc.wantError == nil && err != nil) || (tc.wantError != nil && !errors.Is(err, tc.wantError)) {
				t.Fatalf("UpdateProfileByUserID() = %v, want %v", err, tc.wantError)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
