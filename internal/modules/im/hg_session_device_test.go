package im

import (
	UserJWTMiddlewarePackage "MLC_GO/internal/modules/user/middleware"
	UserServicePackage "MLC_GO/internal/modules/user/service"
	SQL "MLC_GO/internal/pkg/mysql/queries"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/redis/go-redis/v9"
)

// hgSessionHook 在单测中拦截 Redis 命令，不连接共享 Redis，也不记录凭据。
type hgSessionHook struct {
	keys map[string]bool // 会话键是否仍有效；缺失即被踢出。
}

func (h *hgSessionHook) DialHook(next redis.DialHook) redis.DialHook { return next }
func (h *hgSessionHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}
func (h *hgSessionHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		key := cmd.Args()[1].(string)
		switch cmd.Name() {
		case "exists":
			if h.keys[key] {
				cmd.(*redis.IntCmd).SetVal(1)
			}
		default:
			return errors.New("未预期 Redis 命令")
		}
		return nil
	}
}

func TestHGRegisterSessionDeviceEvictsOldest(t *testing.T) {
	for _, tc := range []struct {
		name          string
		oldSession    []byte
		dbFailure     bool
		alreadyActive bool
		ownFailure    bool
	}{
		{name: "成功替换", oldSession: []byte("1234567890123456")},
		{name: "旧设备过渡替换", oldSession: nil},
		{name: "本会话重新绑定设备", alreadyActive: true},
		{name: "本会话撤销失败回滚", alreadyActive: true, ownFailure: true},
		{name: "写入失败回滚", oldSession: []byte("1234567890123456"), dbFailure: true},
		{name: "旧设备撤销失败回滚", oldSession: nil, dbFailure: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			UserServicePackage.HGConfigureSessionDB(db)
			t.Cleanup(func() { UserServicePackage.HGConfigureSessionDB(nil) })
			current := "abcdefabcdefabcdefabcdefabcdefab"
			oldKey := PersistenceRedisPackage.HGAuthSessionKey + "user-1:31323334353637383930313233343536"
			keys := map[string]bool{PersistenceRedisPackage.HGAuthSessionKey + "user-1:" + current: true, oldKey: true}
			hook := &hgSessionHook{keys: keys}
			client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
			client.AddHook(hook)
			defer client.Close()
			mock.ExpectQuery(SQL.HGIMForcedLogoutSQL).WithArgs("user-1", sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"found"}))
			mock.ExpectBegin()
			mock.ExpectQuery(SQL.HGIMLockDeviceOwnerSQL).WithArgs("user-1").WillReturnRows(sqlmock.NewRows([]string{"found"}).AddRow(1))
			mock.ExpectQuery(SQL.HGIMForcedLogoutSQL).WithArgs("user-1", sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"found"}))
			rows := sqlmock.NewRows([]string{"id"})
			for i := 0; i < 5; i++ {
				rows.AddRow([]byte{byte(i)})
			}
			mock.ExpectQuery(SQL.HGIMActiveDevicesSQL).WithArgs("user-1").WillReturnRows(rows)
			activeRows := sqlmock.NewRows([]string{"found"})
			if tc.alreadyActive {
				activeRows.AddRow(1)
			}
			mock.ExpectQuery(SQL.HGIMSessionActiveDeviceSQL).WithArgs("user-1", sqlmock.AnyArg()).WillReturnRows(activeRows)
			oldID := []byte("oldest-device-id")
			if tc.alreadyActive {
				mock.ExpectQuery(SQL.HGIMOwnOldestDeviceSQL).WithArgs("user-1", sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow([]byte("own-session-device")))
			} else {
				mock.ExpectQuery(SQL.HGIMOldestDeviceSQL).WithArgs("user-1").WillReturnRows(sqlmock.NewRows([]string{"id", "session_id"}).AddRow(oldID, tc.oldSession))
			}
			if tc.alreadyActive {
				if tc.ownFailure {
					mock.ExpectExec(SQL.HGIMRevokeOwnDeviceSQL).WithArgs([]byte("own-session-device"), "user-1", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 0))
				} else {
					mock.ExpectExec(SQL.HGIMRevokeOwnDeviceSQL).WithArgs([]byte("own-session-device"), "user-1", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
				}
			} else if len(tc.oldSession) == 0 {
				if tc.dbFailure {
					mock.ExpectExec(SQL.HGIMRevokeLegacyDeviceSQL).WithArgs(oldID, "user-1").WillReturnError(errors.New("MySQL 写入失败"))
				} else {
					mock.ExpectExec(SQL.HGIMRevokeLegacyDeviceSQL).WithArgs(oldID, "user-1").WillReturnResult(sqlmock.NewResult(0, 1))
				}
			} else if tc.dbFailure {
				mock.ExpectExec(SQL.HGIMForceLogoutDeviceSQL).WithArgs("user-1", tc.oldSession).WillReturnError(errors.New("MySQL 写入失败"))
			} else {
				mock.ExpectExec(SQL.HGIMForceLogoutDeviceSQL).WithArgs("user-1", tc.oldSession).WillReturnResult(sqlmock.NewResult(0, 1))
			}
			if !tc.dbFailure && !tc.ownFailure {
				mock.ExpectExec(SQL.HGIMCreateSessionDeviceSQL).WithArgs(sqlmock.AnyArg(), "user-1", sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			} else {
				mock.ExpectRollback()
			}
			credential, err := HGNewService(db, "", nil).HGWithSessionRedis(client).HGRegisterSessionDevice(context.Background(), "user-1", current)
			if (tc.dbFailure || tc.ownFailure) && err == nil {
				t.Fatal("设备撤销失败不得成功")
			}
			if !tc.dbFailure && !tc.ownFailure && (err != nil || len(credential.DeviceID) != 32) {
				t.Fatalf("注册失败: %v", err)
			}
			if !tc.dbFailure && !tc.ownFailure && credential.LegacyReplaced != (!tc.alreadyActive && len(tc.oldSession) == 0) {
				t.Fatal("仅旧版设备替换应报告无法踢出旧登录会话")
			}
			if !keys[oldKey] {
				t.Fatal("会话状态不靠 Redis 更新，强制下线只读 MySQL 权威标记")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// 旧设备过渡成功时必须明确告知客户端：聊天设备已替换，但旧登录会话并未被强制下线。
func TestHGRegisterDeviceLegacyReplacementResponse(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	UserServicePackage.HGConfigureSessionDB(db)
	t.Cleanup(func() { UserServicePackage.HGConfigureSessionDB(nil) })
	sessionID := "abcdefabcdefabcdefabcdefabcdefab"
	hook := &hgSessionHook{keys: map[string]bool{PersistenceRedisPackage.HGAuthSessionKey + "user-1:" + sessionID: true}}
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	client.AddHook(hook)
	defer client.Close()
	mock.ExpectQuery(SQL.HGIMForcedLogoutSQL).WithArgs("user-1", sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"found"}))
	mock.ExpectBegin()
	mock.ExpectQuery(SQL.HGIMLockDeviceOwnerSQL).WithArgs("user-1").WillReturnRows(sqlmock.NewRows([]string{"found"}).AddRow(1))
	mock.ExpectQuery(SQL.HGIMForcedLogoutSQL).WithArgs("user-1", sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"found"}))
	rows := sqlmock.NewRows([]string{"id"})
	for i := 0; i < 5; i++ {
		rows.AddRow([]byte{byte(i)})
	}
	mock.ExpectQuery(SQL.HGIMActiveDevicesSQL).WithArgs("user-1").WillReturnRows(rows)
	mock.ExpectQuery(SQL.HGIMSessionActiveDeviceSQL).WithArgs("user-1", sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"found"}))
	oldID := []byte("oldest-device-id")
	mock.ExpectQuery(SQL.HGIMOldestDeviceSQL).WithArgs("user-1").WillReturnRows(sqlmock.NewRows([]string{"id", "session_id"}).AddRow(oldID, nil))
	mock.ExpectExec(SQL.HGIMRevokeLegacyDeviceSQL).WithArgs(oldID, "user-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(SQL.HGIMCreateSessionDeviceSQL).WithArgs(sqlmock.AnyArg(), "user-1", sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/im/device/register", strings.NewReader("{}"))
	request = request.WithContext(context.WithValue(request.Context(), UserJWTMiddlewarePackage.UserIDKey, &UserServicePackage.HGClaims{UserID: "user-1", SessionID: sessionID}))
	response := httptest.NewRecorder()
	HGNewHandler(HGNewService(db, "", nil).HGWithSessionRedis(client)).HGRegisterDevice(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("过渡替换注册失败: %d", response.Code)
	}
	var result struct {
		Code   int `json:"code"`
		Result struct {
			DeviceID       string `json:"deviceId"`
			Secret         string `json:"secret"`
			LegacyReplaced bool   `json:"legacyReplaced"`
		} `json:"result"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Code != 200 || len(result.Result.DeviceID) != 32 || len(result.Result.Secret) != 64 || !result.Result.LegacyReplaced {
		t.Fatal("注册响应必须返回设备凭据并标明旧会话未被踢出")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
