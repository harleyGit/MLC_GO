package UserServicePackage

import (
	SQL "MLC_GO/internal/pkg/mysql/queries"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

// hgAuthHook 模拟登录会话和 token 状态；拦截全部命令以保证不触达共享 Redis。
type hgAuthHook struct{ keys map[string]bool }

func (h *hgAuthHook) DialHook(next redis.DialHook) redis.DialHook { return next }
func (h *hgAuthHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}
func (h *hgAuthHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		key := cmd.Args()[1].(string)
		switch cmd.Name() {
		case "set":
			h.keys[key] = true
			cmd.(*redis.StatusCmd).SetVal("OK")
		case "exists":
			if h.keys[key] {
				cmd.(*redis.IntCmd).SetVal(1)
			}
		case "del":
			delete(h.keys, key)
			cmd.(*redis.IntCmd).SetVal(1)
		case "expire":
			if h.keys[key] {
				cmd.(*redis.BoolCmd).SetVal(true)
			}
		default:
			return errors.New("未预期 Redis 命令")
		}
		return nil
	}
}

func TestHGSessionRevocationBlocksRefresh(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	HGConfigureSessionDB(db)
	t.Cleanup(func() { HGConfigureSessionDB(nil) })
	hook := &hgAuthHook{keys: make(map[string]bool)}
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	client.AddHook(hook)
	defer client.Close()
	pair, err := GenerateTokens(ctx, client, "user-1", "", "user")
	if err != nil {
		t.Fatal(err)
	}
	claims := &HGClaims{}
	_, err = jwt.ParseWithClaims(pair.AccessToken, claims, func(t *jwt.Token) (any, error) { return Secret, nil })
	if err != nil || len(claims.SessionID) != 32 {
		t.Fatalf("未签发有效会话 ID: %v", err)
	}
	mock.ExpectQuery(SQL.HGIMForcedLogoutSQL).WithArgs("user-1", sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"found"}))
	active, err := HGSessionActive(ctx, client, "user-1", claims.SessionID)
	if err != nil || !active {
		t.Fatalf("新登录会话应有效: %v", err)
	}
	mock.ExpectQuery(SQL.HGIMForcedLogoutSQL).WithArgs("user-1", sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"found"}))
	refreshed, err := RefreshToken(ctx, client, pair.RefreshToken)
	if err != nil {
		t.Fatalf("有效会话应能续签: %v", err)
	}
	refreshClaims := &HGClaims{}
	_, err = jwt.ParseWithClaims(refreshed.AccessToken, refreshClaims, func(t *jwt.Token) (any, error) { return Secret, nil })
	if err != nil || refreshClaims.SessionID != claims.SessionID {
		t.Fatalf("续签不得更换会话归属: %v", err)
	}
	mock.ExpectQuery(SQL.HGIMForcedLogoutSQL).WithArgs("user-1", sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"found"}).AddRow(1))
	active, err = HGSessionActive(ctx, client, "user-1", claims.SessionID)
	if err != nil || active {
		t.Fatalf("MySQL 标记强制下线后 access 应立即失效: %v", err)
	}
	delete(hook.keys, PersistenceRedisPackage.HGAuthSessionKey+"user-1:"+claims.SessionID)
	active, err = HGSessionActive(ctx, client, "user-1", claims.SessionID)
	if err != nil || active {
		t.Fatalf("被撤销的 access 会话不能通过鉴权: %v", err)
	}
	if _, err = RefreshToken(ctx, client, refreshed.RefreshToken); err == nil {
		t.Fatal("被撤销的 refresh token 不得续签")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
