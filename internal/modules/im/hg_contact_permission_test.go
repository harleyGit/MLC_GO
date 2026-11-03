package im

import (
	SQL "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// 历史会话允许保留，但不存在双方同意的关系时禁止创建新普通单聊。
func TestHGCreateConversationRequiresFriendship(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(SQL.HGIMUserExistsSQL).WithArgs("B").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(1))
	mock.ExpectBegin()
	mock.ExpectQuery(SQL.HGIMContactPairSQL).WithArgs("A", "B").WillReturnRows(sqlmock.NewRows([]string{"exists"}))
	mock.ExpectRollback()
	_, err = HGNewService(db, "topic", nil).HGCreateConversation(context.Background(), "A", "B")
	if !errors.Is(err, HGErrForbidden) {
		t.Fatalf("未建立好友关系应禁止会话: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
