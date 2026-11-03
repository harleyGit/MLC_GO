package UserServicePackage

import (
	userrepository "MLC_GO/internal/modules/user/repository"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql/driver"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestGetUserList_ReturnsProfileFieldsByCursor(t *testing.T) {
	for _, hgCase := range []struct {
		name       string
		cursor     int64
		query      string
		args       []any
		firstID    int64
		wantCursor int64
		wantPublic string
		wantSecond string
	}{
		{name: "首屏", cursor: 0, query: SQLQueriesPackage.QueryUserPageFirstSQL, args: []any{3}, firstID: 26, wantCursor: 25, wantPublic: "U00000000000000000026", wantSecond: "U00000000000000000025"},
		{name: "下一页", cursor: 26, query: SQLQueriesPackage.QueryUserPageV2SQL, args: []any{int64(26), 3}, firstID: 25, wantCursor: 24, wantPublic: "U00000000000000000025", wantSecond: "U00000000000000000024"},
	} {
		t.Run(hgCase.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()

			rows := sqlmock.NewRows([]string{"id", "user_id", "user_name", "nickname", "signature", "gender", "birth_month", "avatar_url", "email", "phone", "created_at", "updated_at"}).
				AddRow(hgCase.firstID, "internal-alice", "alice", "小明", "签名", 1, time.Date(2000, 3, 1, 0, 0, 0, 0, time.UTC), "/uploads/user/avatar.png", "user@example.test", "13800000000", "2026-01-01", "2026-02-01").
				AddRow(hgCase.firstID-1, "internal-bob", "bob", nil, nil, 0, nil, nil, nil, nil, "2026-01-01", "2026-02-01").
				AddRow(hgCase.firstID-2, "internal-charlie", "charlie", nil, nil, 0, nil, nil, nil, nil, "2026-01-01", "2026-02-01")
			hgArgs := make([]driver.Value, len(hgCase.args))
			for i, arg := range hgCase.args {
				hgArgs[i] = arg
			}
			mock.ExpectQuery(regexp.QuoteMeta(hgCase.query)).WithArgs(hgArgs...).WillReturnRows(rows)
			mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.UserTotalNumSQL)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))

			svc := NewUserService(userrepository.NewUserRepo(db), nil, nil)
			response, err := svc.GetUserList(context.Background(), hgCase.cursor, 2)
			if err != nil {
				t.Fatalf("GetUserList: %v", err)
			}
			if len(response.Result) != 2 || !response.HasMore || response.NextCursor != hgCase.wantCursor || response.NumResults != 3 {
				t.Fatalf("分页结果不符: %+v", response)
			}
			encoded, err := json.Marshal(response)
			if err != nil {
				t.Fatal(err)
			}
			text := string(encoded)
			for _, field := range []string{`"user_id":"internal-alice"`, `"avatar_url":"/uploads/user/avatar.png"`, `"nickname":"小明"`, `"signature":"签名"`, `"gender":1`, `"birth_month":"2000-03-01"`, `"created_at":"2026-01-01"`} {
				if !strings.Contains(text, field) {
					t.Errorf("响应缺少 %s: %s", field, text)
				}
			}
			if response.Result[0].PublicID == nil || *response.Result[0].PublicID != hgCase.wantPublic || !strings.Contains(text, `"publicId":"`+hgCase.wantPublic+`"`) {
				t.Errorf("首个用户 publicId 错误: %s", text)
			}
			if response.Result[1].PublicID == nil || *response.Result[1].PublicID != hgCase.wantSecond {
				t.Errorf("第二个用户 publicId 错误: %s", text)
			}
			if strings.Contains(text, "passwordHash") || strings.Contains(text, `"salt"`) {
				t.Errorf("列表不应包含认证秘密: %s", text)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
