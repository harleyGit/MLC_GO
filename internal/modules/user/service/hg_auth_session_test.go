package UserServicePackage

import (
	SQL "MLC_GO/internal/pkg/mysql/queries"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

// hgAuthHook 模拟会话索引、刷新状态及过期设置；拦截全部命令以保证不触达共享 Redis。
type hgAuthHook struct {
	keys     map[string]bool
	ttl      map[string]time.Duration
	sessions []string
	refresh  map[string]string
}

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
		case "eval":
			args := cmd.Args()
			script := args[1].(string)
			index, session, refresh, fence := args[3].(string), args[4].(string), args[5].(string), args[6].(string)
			ttl := time.Duration(args[9].(int)) * time.Second
			if h.keys[fence] || (strings.Contains(script, "local jti = redis.call('GET', KEYS[2])") && (!h.keys[session] || !h.keys[refresh])) {
				cmd.(*redis.Cmd).SetVal(int64(0))
				break
			}
			h.keys[session], h.keys[refresh], h.keys[index] = true, true, true
			h.ttl[session], h.ttl[refresh], h.ttl[index] = ttl, ttl, ttl
			h.refresh[session] = refresh
			for i, old := range h.sessions {
				if old == session {
					h.sessions = append(h.sessions[:i], h.sessions[i+1:]...)
					break
				}
			}
			h.sessions = append(h.sessions, session)
			if len(h.sessions) > 100 {
				old := h.sessions[0]
				h.sessions = h.sessions[1:]
				delete(h.keys, old)
				delete(h.keys, h.refresh[old])
			}
			cmd.(*redis.Cmd).SetVal(int64(1))
		case "exists":
			if h.keys[key] {
				cmd.(*redis.IntCmd).SetVal(1)
			}
		case "del":
			delete(h.keys, key)
			cmd.(*redis.IntCmd).SetVal(1)
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
	hook := &hgAuthHook{keys: make(map[string]bool), ttl: make(map[string]time.Duration), refresh: make(map[string]string)}
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
	if lifetime := claims.ExpiresAt.Sub(claims.IssuedAt.Time); lifetime != AccessTTL {
		t.Fatalf("初次 access 有效期应为 24 小时: %v", lifetime)
	}
	originalRefresh := &HGClaims{}
	if _, err := jwt.ParseWithClaims(pair.RefreshToken, originalRefresh, func(t *jwt.Token) (any, error) { return Secret, nil }); err != nil || originalRefresh.ExpiresAt != nil {
		t.Fatalf("refresh 过期应由服务端 30 天闲置 TTL 控制: %v", err)
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
	if refreshed.RefreshToken != pair.RefreshToken || refreshed.AccessToken == pair.AccessToken {
		t.Fatal("续签必须返回原 refresh 与新 access")
	}
	if lifetime := refreshClaims.ExpiresAt.Sub(refreshClaims.IssuedAt.Time); lifetime != AccessTTL {
		t.Fatalf("续签 access 有效期应为 24 小时: %v", lifetime)
	}
	if ttl := hook.ttl[PersistenceRedisPackage.AuthRefreshKey+"user-1:"+originalRefresh.JTI]; ttl != RefreshTTL {
		t.Fatalf("refresh TTL 应滑动重置至 30 天: %v", ttl)
	}
	mock.ExpectQuery(SQL.HGIMForcedLogoutSQL).WithArgs("user-1", sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"found"}))
	again, err := RefreshToken(ctx, client, pair.RefreshToken)
	if err != nil || again.RefreshToken != pair.RefreshToken || again.AccessToken == refreshed.AccessToken {
		t.Fatalf("同一 refresh 必须可重复续期且每次签发新 access: %v", err)
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

// 先前 access 不因续签删除；服务端 refresh 状态过期后即使 JWT 仍签名有效也不能复活会话。
func TestHGRefreshIdleExpiryAndOldAccess(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	HGConfigureSessionDB(db)
	t.Cleanup(func() { HGConfigureSessionDB(nil) })
	hook := &hgAuthHook{keys: make(map[string]bool), ttl: make(map[string]time.Duration), refresh: make(map[string]string)}
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	client.AddHook(hook)
	defer client.Close()
	pair, err := GenerateTokens(ctx, client, "user-2", "", "user")
	if err != nil {
		t.Fatal(err)
	}
	claims := &HGClaims{}
	if _, err := jwt.ParseWithClaims(pair.RefreshToken, claims, func(t *jwt.Token) (any, error) { return Secret, nil }); err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery(SQL.HGIMForcedLogoutSQL).WithArgs("user-2", sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"found"}))
	if _, err := RefreshToken(ctx, client, pair.RefreshToken); err != nil {
		t.Fatal(err)
	}
	oldAccessClaims := &HGClaims{}
	if _, err := jwt.ParseWithClaims(pair.AccessToken, oldAccessClaims, func(t *jwt.Token) (any, error) { return Secret, nil }); err != nil ||
		!hook.keys[PersistenceRedisPackage.HGAuthSessionKey+"user-2:"+claims.SessionID] {
		t.Fatalf("旧 access 仍须在原有效期及会话存活期间有效: %v", err)
	}
	delete(hook.keys, PersistenceRedisPackage.AuthRefreshKey+"user-2:"+claims.JTI)
	mock.ExpectQuery(SQL.HGIMForcedLogoutSQL).WithArgs("user-2", sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"found"}))
	if _, err := RefreshToken(ctx, client, pair.RefreshToken); err == nil {
		t.Fatal("闲置超过 30 天后 refresh 状态到期不得续期")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 同一站点视为单一应用；第 101 次登录须淘汰最久未续期的授权，不能只停止发新 token。
func TestHGAuthSessionCapEvictsOldest(t *testing.T) {
	ctx := context.Background()
	hook := &hgAuthHook{keys: make(map[string]bool), ttl: make(map[string]time.Duration), refresh: make(map[string]string)}
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	client.AddHook(hook)
	defer client.Close()
	var first *TokenPair
	for i := 0; i < 101; i++ {
		pair, err := GenerateTokens(ctx, client, "user-3", "", "user")
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = pair
		}
	}
	if len(hook.sessions) != 100 {
		t.Fatalf("每用户至多 100 个有效会话: %d", len(hook.sessions))
	}
	claims := &HGClaims{}
	if _, err := jwt.ParseWithClaims(first.RefreshToken, claims, func(t *jwt.Token) (any, error) { return Secret, nil }); err != nil {
		t.Fatal(err)
	}
	if hook.keys[PersistenceRedisPackage.HGAuthSessionKey+"user-3:"+claims.SessionID] ||
		hook.keys[PersistenceRedisPackage.AuthRefreshKey+"user-3:"+claims.JTI] {
		t.Fatal("最久未续期的会话及 refresh 状态应被一起淘汰")
	}
	if _, err := RefreshToken(ctx, client, first.RefreshToken); err == nil {
		t.Fatal("淘汰的授权不得再次刷新")
	}
}
