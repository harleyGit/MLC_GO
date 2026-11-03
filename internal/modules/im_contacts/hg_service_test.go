package im_contacts

import (
	SQL "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

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
		if got := hgPublicUsername(value); got != "" {
			t.Fatalf("不应公开账号名 %q: %q", value, got)
		}
	}
	if got := hgPublicUsername("zhangsan888"); got != "zhangsan888" {
		t.Fatalf("合法 ID 不应被屏蔽: %q", got)
	}
}

// 五类精确查找均只投影公开资料；手机号/邮箱即使用于点查也不得出现在返回 JSON 中。
func TestHGSearchExactIdentityDoesNotReturnPrivateInput(t *testing.T) {
	for _, tc := range []struct {
		name, kind, keyword, query string
		arg                        any
	}{
		{"系统 ID", "public_id", "U00000000000000000001", SQL.HGIMContactSearchPublicSQL, uint64(1)},
		{"目标用户 ID", "targetUserId", "HGUSR_TW_01ABCDEFGHJKMNPQRSTVWXYZ12", SQL.HGIMContactProfileSQL, "HGUSR_TW_01ABCDEFGHJKMNPQRSTVWXYZ12"},
		{"手机号", "phone", "13800138000", SQL.HGIMContactSearchPhoneSQL, "13800138000"},
		{"邮箱", "email", "user@example.com", SQL.HGIMContactSearchEmailSQL, "user@example.com"},
		{"兼容自定义 ID", "username", "example_user", SQL.HGIMContactSearchUsernameSQL, "example_user"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			mock.ExpectQuery(tc.query).WithArgs(tc.arg).WillReturnRows(sqlmock.NewRows([]string{"user_id", "public_id", "username", "nickname", "signature", "avatar_url"}).AddRow("target-id", "U00000000000000000001", "example_user", "测试用户", "", "/uploads/avatar.png"))
			low, high := hgPair("viewer-id", "target-id")
			mock.ExpectQuery(SQL.HGIMContactStatusSQL).WithArgs(low, high, "viewer-id", "target-id", "target-id", "viewer-id", "viewer-id", "target-id", "target-id", "viewer-id").WillReturnRows(sqlmock.NewRows([]string{"friend", "blocked", "outgoing", "incoming"}).AddRow(false, false, false, false))
			profile, err := HGNewService(db).HGSearch(context.Background(), "viewer-id", tc.kind, tc.keyword)
			if err != nil || profile.FriendStatus != "none" || profile.AvatarURL != "/uploads/avatar.png" {
				t.Fatalf("精确查找失败: %v, status=%s", err, profile.FriendStatus)
			}
			encoded, err := json.Marshal(profile)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(encoded), "13800138000") || strings.Contains(string(encoded), "user@example.com") || strings.Contains(string(encoded), `"email"`) || strings.Contains(string(encoded), `"phone"`) {
				t.Fatal("公开资料不得回传手机号或邮箱")
			}
			if !strings.Contains(string(encoded), `"avatarUrl":"/uploads/avatar.png"`) {
				t.Fatal("搜索响应缺少真实头像 URL")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// 好友、申请和黑名单共用公开头像字段；未上传头像的数据库 NULL 返回空串，不增加逐用户查询。
func TestHGContactListsIncludeAvatar(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := HGNewService(db)
	mock.ExpectQuery(SQL.HGIMContactFriendsSQL).WithArgs("viewer", "viewer", "viewer", "viewer", uint64(0), 100).WillReturnRows(
		sqlmock.NewRows([]string{"user_id", "public_id", "username", "nickname", "signature", "avatar_url", "remark", "tags", "phone", "description"}).AddRow("friend", "U00000000000000000001", "friend_id", "好友", "", "/uploads/avatar.png", "", "[]", "", ""))
	friends, err := service.HGList(context.Background(), "viewer", 0)
	if err != nil || len(friends) != 1 || friends[0].AvatarURL != "/uploads/avatar.png" {
		t.Fatalf("好友头像读取失败: %v, %+v", err, friends)
	}
	mock.ExpectQuery(SQL.HGIMContactRequestsSQL).WithArgs("viewer", uint64(0), 100).WillReturnRows(
		sqlmock.NewRows([]string{"request_id", "user_id", "public_id", "username", "nickname", "avatar_url", "message", "created_at"}).AddRow("req", "sender", "U00000000000000000002", "sender_id", "申请人", "/uploads/sender.png", "你好", time.Now()))
	requests, err := service.HGIncoming(context.Background(), "viewer", 0)
	if err != nil || len(requests) != 1 || requests[0].AvatarURL != "/uploads/sender.png" {
		t.Fatalf("申请人头像读取失败: %v, %+v", err, requests)
	}
	mock.ExpectQuery(SQL.HGIMContactBlocksSQL).WithArgs("viewer", uint64(0), 100).WillReturnRows(
		sqlmock.NewRows([]string{"user_id", "public_id", "username", "nickname", "avatar_url", "block_note", "created_at"}).AddRow("blocked", "U00000000000000000003", "blocked_id", "黑名单", "", "", time.Now()))
	blocks, err := service.HGBlocks(context.Background(), "viewer", 0)
	if err != nil || len(blocks) != 1 || blocks[0].AvatarURL != "" {
		t.Fatalf("空头像应保留占位: %v, %+v", err, blocks)
	}
	mock.ExpectQuery(SQL.HGIMContactProfileSQL).WithArgs("friend").WillReturnRows(
		sqlmock.NewRows([]string{"user_id", "public_id", "username", "nickname", "signature", "avatar_url"}).AddRow("friend", "U00000000000000000001", "friend_id", "好友", "", "/uploads/avatar.png"))
	profile, err := service.HGProfileFor(context.Background(), "friend")
	if err != nil || profile.AvatarURL != "/uploads/avatar.png" {
		t.Fatalf("资料头像读取失败: %v, %+v", err, profile)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 非法或未知搜索类型必须在查询数据库前失败，避免意外枚举与宽泛扫描。
func TestHGSearchRejectsInvalidIdentityWithoutQuery(t *testing.T) {
	for _, tc := range []struct{ kind, keyword string }{
		{"phone", "1380013800x"}, {"phone", "1380013800"}, {"email", "user@localhost"},
		{"email", "user@example.com\n"}, {"targetUserId", strings.Repeat("a", 129)},
		{"public_id", "U00000000000000000000"}, {"unknown", "test"},
	} {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		_, err = HGNewService(db).HGSearch(context.Background(), "viewer-id", tc.kind, tc.keyword)
		if !errors.Is(err, HGErrInvalid) {
			t.Fatalf("%s 应拒绝非法关键词: %v", tc.kind, err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		db.Close()
	}
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
	mock.ExpectQuery(SQL.HGIMContactProfileSQL).WithArgs("A").WillReturnRows(sqlmock.NewRows([]string{"user_id", "public_id", "username", "nickname", "signature", "avatar_url"}).AddRow("A", "U00000000000000000001", "a", "甲", "", "/uploads/avatar.png"))
	mock.ExpectCommit()
	friend, err := HGNewService(db).HGDecide(context.Background(), "B", id, true)
	if err != nil || friend.UserID != "A" || friend.AvatarURL != "/uploads/avatar.png" {
		t.Fatalf("申请接受结果: %+v, %v", friend, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
