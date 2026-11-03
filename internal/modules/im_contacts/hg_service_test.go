package im_contacts

import (
	SQL "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// 确认申请不能在事务外创建好友；所有用户 ID 必须从 JWT 传入。
func TestHGSendRequestRejectsSelf(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := HGNewService(db).HGSendRequest(context.Background(), "A", "A", "hello"); !errors.Is(err, HGErrInvalid) {
		t.Fatalf("自加好友应拒绝: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 遗留账号名可能含电话或邮箱，禁止当作公开 ID 返回或提供精确搜索入口。
func TestHGPublicUsernameDoesNotExposePrivateIdentity(t *testing.T) {
	for _, value := range []string{"13800138000", "user@example.com", "ab", "a/bc"} {
		if got := hgPublicUsername(value); got != "" { t.Fatalf("不应公开账号名 %q: %q", value, got) }
	}
	if got := hgPublicUsername("zhangsan888"); got != "zhangsan888" { t.Fatalf("合法 ID 不应被屏蔽: %q", got) }
}

// 反向申请只有接收者可以决定；不触发任何写入。
func TestHGDecideRejectsOtherUser(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(SQL.HGIMContactRequestSQL).WithArgs("00000000-0000-0000-0000-000000000000").WillReturnRows(sqlmock.NewRows([]string{"requester_id", "addressee_id", "status"}).AddRow("A", "B", 0))
	_, err = HGNewService(db).HGDecide(context.Background(), "C", "00000000-0000-0000-0000-000000000000", true)
	if !errors.Is(err, HGErrDenied) {
		t.Fatalf("非接收者应拒绝: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 黑名单有方向；解除拉黑只删自己的黑名单行，不会恢复友情。
func TestHGUnblockDoesNotRestoreFriendship(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	for _, id := range []string{"A", "B"} {
		mock.ExpectQuery(SQL.HGIMContactLockUserSQL).WithArgs(id).WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	}
	mock.ExpectExec(SQL.HGIMContactUnblockSQL).WithArgs("B", "A").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := HGNewService(db).HGSetBlock(context.Background(), "B", "A", false); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 接受申请必须在同一事务内建立双向好友、会话和双方成员；成功后才返回申请人公开资料。
func TestHGAcceptCreatesFriendAndConversationAtomically(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	id := "00000000-0000-0000-0000-000000000000"
	request := func() *sqlmock.Rows {
		return sqlmock.NewRows([]string{"requester_id", "addressee_id", "status"}).AddRow("A", "B", 0)
	}
	mock.ExpectQuery(SQL.HGIMContactRequestSQL).WithArgs(id).WillReturnRows(request())
	mock.ExpectBegin()
	for _, user := range []string{"A", "B"} {
		mock.ExpectQuery(SQL.HGIMContactLockUserSQL).WithArgs(user).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(1))
	}
	mock.ExpectQuery(SQL.HGIMContactLockRequestSQL).WithArgs(id).WillReturnRows(request())
	mock.ExpectQuery(SQL.HGIMContactBlockPairSQL).WithArgs("A", "B", "B", "A").WillReturnRows(sqlmock.NewRows([]string{"exists"}))
	mock.ExpectExec(SQL.HGIMContactAcceptPairSQL).WithArgs("A", "B").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(SQL.HGIMContactDecideRequestSQL).WithArgs(1, id).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(SQL.HGIMCreateConversationSQL).WithArgs("A", "B").WillReturnResult(sqlmock.NewResult(42, 1))
	for _, user := range []string{"A", "B"} {
		mock.ExpectExec(SQL.HGIMAddMemberSQL).WithArgs(int64(42), user).WillReturnResult(sqlmock.NewResult(0, 1))
	}
	mock.ExpectQuery(SQL.HGIMContactProfileSQL).WithArgs("A").WillReturnRows(sqlmock.NewRows([]string{"user_id", "public_id", "username", "nickname", "signature"}).AddRow("A", "U00000000000000000001", "a", "甲", ""))
	mock.ExpectCommit()
	friend, err := HGNewService(db).HGDecide(context.Background(), "B", id, true)
	if err != nil || friend.UserID != "A" {
		t.Fatalf("申请接受结果: %+v, %v", friend, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
