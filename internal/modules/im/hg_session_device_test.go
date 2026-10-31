package im

import (
	UserServicePackage "MLC_GO/internal/modules/user/service"
	SQL "MLC_GO/internal/pkg/mysql/queries"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"context"
	"errors"
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
		wantConflict  bool
		alreadyActive bool
	}{
		{name: "成功替换", oldSession: []byte("1234567890123456")},
		{name: "旧设备无会话", wantConflict: true},
		{name: "重复注册不得连续踢人", wantConflict: true, alreadyActive: true},
		{name: "写入失败回滚", oldSession: []byte("1234567890123456"), dbFailure: true},
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
			if !tc.alreadyActive {
				mock.ExpectQuery(SQL.HGIMOldestDeviceSQL).WithArgs("user-1").WillReturnRows(sqlmock.NewRows([]string{"id", "session_id"}).AddRow(oldID, tc.oldSession))
			}
			if !tc.wantConflict && !tc.dbFailure {
				mock.ExpectExec(SQL.HGIMForceLogoutDeviceSQL).WithArgs("user-1", tc.oldSession).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec(SQL.HGIMCreateSessionDeviceSQL).WithArgs(sqlmock.AnyArg(), "user-1", sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			} else {
				if tc.dbFailure {
					mock.ExpectExec(SQL.HGIMForceLogoutDeviceSQL).WithArgs("user-1", tc.oldSession).WillReturnError(errors.New("MySQL 写入失败"))
				}
				mock.ExpectRollback()
			}
			credential, err := HGNewService(db, "", nil).HGWithSessionRedis(client).HGRegisterSessionDevice(context.Background(), "user-1", current)
			if tc.wantConflict && !errors.Is(err, HGErrConflict) {
				t.Fatalf("旧记录应保持冲突: %v", err)
			}
			if tc.dbFailure && err == nil {
				t.Fatal("MySQL 强制下线失败不得成功")
			}
			if !tc.wantConflict && !tc.dbFailure && (err != nil || len(credential.DeviceID) != 32) {
				t.Fatalf("注册失败: %v", err)
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
