package UserServicePackage

import (
	Mail "MLC_GO/internal/pkg/email"
	SQL "MLC_GO/internal/pkg/mysql/queries"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

func hgIsolatedAuthRedis(t *testing.T) *redis.Client {
	t.Helper()
	if os.Getenv("MLC_AUTH_SESSION_REDIS_INTEGRATION") != "1" {
		t.Skip("需要显式开启隔离 Redis 集成测试")
	}
	server, err := exec.LookPath("redis-server")
	if err != nil {
		t.Skip("缺少 redis-server")
	}
	// macOS Unix socket 路径长度有限制，t.TempDir() 含长测试名会超过上限。
	dir, err := os.MkdirTemp("", "hg-auth-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socket := filepath.Join(dir, "auth.sock")
	cmd := exec.Command(server, "--port", "0", "--unixsocket", socket, "--unixsocketperm", "700", "--save", "", "--appendonly", "no", "--dir", dir)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	rdb := redis.NewClient(&redis.Options{Network: "unix", Addr: socket, MaxRetries: 0})
	t.Cleanup(func() { _ = rdb.Close() })
	ctx := context.Background()
	ready := false
	for i := 0; i < 100; i++ {
		if rdb.Ping(ctx).Err() == nil {
			ready = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !ready {
		t.Fatal("隔离 Redis 未在启动预算内就绪")
	}
	return rdb
}

// 隔离 Redis 实测 Lua 的原子上限与滑动 TTL；默认不启动外部进程。
func TestHGAuthSessionRedisIntegration(t *testing.T) {
	rdb := hgIsolatedAuthRedis(t)
	ctx := context.Background()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	HGConfigureSessionDB(db)
	t.Cleanup(func() { HGConfigureSessionDB(nil) })

	first, err := hgNewSessionTokens(ctx, rdb, "user-integration", "", "user", 0)
	if err != nil {
		t.Fatal(err)
	}
	refreshClaims := &HGClaims{}
	if _, err := jwt.ParseWithClaims(first.RefreshToken, refreshClaims, func(t *jwt.Token) (any, error) { return Secret, nil }); err != nil {
		t.Fatal(err)
	}
	refreshKey := PersistenceRedisPackage.AuthRefreshKey + "user-integration:" + refreshClaims.JTI
	if ttl := rdb.TTL(ctx, refreshKey).Val(); ttl < RefreshTTL-time.Minute || ttl > RefreshTTL {
		t.Fatalf("首次 refresh 状态 TTL 不正确: %v", ttl)
	}
	hgExpectSessionAuthority(mock, "user-integration", 0, false)
	refreshed, err := RefreshToken(ctx, rdb, first.RefreshToken)
	if err != nil || refreshed.RefreshToken != first.RefreshToken || refreshed.AccessToken == first.AccessToken {
		t.Fatalf("真实 Redis 续签不正确: %v", err)
	}
	if ttl := rdb.TTL(ctx, refreshKey).Val(); ttl < RefreshTTL-time.Minute {
		t.Fatalf("刷新没有重置 30 天 TTL: %v", ttl)
	}
	// 多个请求复用同一 refresh，不因轮换/删除导致只有首个调用成功。
	const parallelRefreshes = 8
	for i := 0; i < parallelRefreshes; i++ {
		hgExpectSessionAuthority(mock, "user-integration", 0, false)
	}
	var wg sync.WaitGroup
	results := make(chan *TokenPair, parallelRefreshes)
	errs := make(chan error, parallelRefreshes)
	for i := 0; i < parallelRefreshes; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pair, refreshErr := RefreshToken(ctx, rdb, first.RefreshToken)
			results <- pair
			errs <- refreshErr
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	accessTokens := make(map[string]bool)
	for pair := range results {
		if pair != nil {
			if pair.RefreshToken != first.RefreshToken || accessTokens[pair.AccessToken] {
				t.Fatal("并发续签应保留原 refresh 且新 access 各不相同")
			}
			accessTokens[pair.AccessToken] = true
		}
	}
	for refreshErr := range errs {
		if refreshErr != nil {
			t.Fatalf("并发续签失败: %v", refreshErr)
		}
	}
	if len(accessTokens) != parallelRefreshes {
		t.Fatalf("并发续签仅成功 %d/%d", len(accessTokens), parallelRefreshes)
	}
	// 上线前签发的 JWT 即使固定 exp 已过，也仅能在成功续过期、Redis 状态仍在时继续刷新。
	legacyClaims := *refreshClaims
	legacyClaims.RegisteredClaims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Hour))
	legacyToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, &legacyClaims).SignedString(Secret)
	if err != nil {
		t.Fatal(err)
	}
	hgExpectSessionAuthority(mock, "user-integration", 0, false)
	legacyRefreshed, err := RefreshToken(ctx, rdb, legacyToken)
	if err != nil || legacyRefreshed.RefreshToken != legacyToken {
		t.Fatalf("已延长 Redis TTL 的旧版 refresh 应原值续签: %v", err)
	}
	if _, err := RefreshToken(ctx, rdb, first.AccessToken); err == nil {
		t.Fatal("access token 不能充当 refresh token")
	}
	var oldestActive, newest *TokenPair
	for i := 0; i < 100; i++ {
		pair, err := hgNewSessionTokens(ctx, rdb, "user-integration", "", "user", 0)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			oldestActive = pair
		}
		newest = pair
	}
	indexKey := PersistenceRedisPackage.HGAuthSessionIndexKey + "user-integration"
	if count := rdb.ZCard(ctx, indexKey).Val(); count != 100 {
		t.Fatalf("实际 Redis 里应仅有 100 个有效授权: %d", count)
	}
	if n := rdb.Exists(ctx, refreshKey, PersistenceRedisPackage.HGAuthSessionKey+"user-integration:"+refreshClaims.SessionID).Val(); n != 0 {
		t.Fatal("超过 100 个授权后旧会话及 refresh 状态必须一并撤销")
	}
	if _, err := RefreshToken(ctx, rdb, first.RefreshToken); err == nil {
		t.Fatal("淘汰的 refresh 不得重新创建会话")
	}
	newestClaims := &HGClaims{}
	if _, err := jwt.ParseWithClaims(newest.RefreshToken, newestClaims, func(*jwt.Token) (any, error) { return Secret, nil }); err != nil {
		t.Fatal(err)
	}
	if err := HGRevokeAuthSession(ctx, rdb, "user-integration", newestClaims.SessionID); err != nil {
		t.Fatal(err)
	}
	if n := rdb.Exists(ctx, PersistenceRedisPackage.AuthRefreshKey+"user-integration:"+newestClaims.JTI,
		PersistenceRedisPackage.HGAuthSessionKey+"user-integration:"+newestClaims.SessionID).Val(); n != 0 {
		t.Fatal("登出必须即时撤销会话和 refresh")
	}
	if _, err := hgNewSessionTokens(ctx, rdb, "user-integration", "", "user", 0); err != nil {
		t.Fatal(err)
	}
	if n := rdb.ZCard(ctx, indexKey).Val(); n != 100 {
		t.Fatalf("登出后重新登录应保留 100 个有效会话: %d", n)
	}
	oldestClaims := &HGClaims{}
	if _, err := jwt.ParseWithClaims(oldestActive.RefreshToken, oldestClaims, func(*jwt.Token) (any, error) { return Secret, nil }); err != nil {
		t.Fatal(err)
	}
	if n := rdb.Exists(ctx, PersistenceRedisPackage.HGAuthSessionKey+"user-integration:"+oldestClaims.SessionID).Val(); n != 1 {
		t.Fatal("登出最近会话后重新登录不得提前淘汰最早的有效会话")
	}
	// 模拟 MySQL 恢复事务提交，故意保留 Redis 旧会话；旧 access/refresh 仍必须拒绝。
	hgExpectSessionAuthority(mock, "user-integration", 1, false)
	if active, err := HGSessionActive(ctx, rdb, "user-integration", oldestClaims.SessionID); err != nil || active {
		t.Fatalf("代际推进后未清理的旧 access 仍可用: %v", err)
	}
	hgExpectSessionAuthority(mock, "user-integration", 1, false)
	if _, err := RefreshToken(ctx, rdb, oldestActive.RefreshToken); err == nil {
		t.Fatal("恢复前的 refresh 不得抬升会话代际")
	}
	mock.ExpectQuery(SQL.HGSelectRecoveryGeneration).WithArgs("user-integration").
		WillReturnRows(sqlmock.NewRows([]string{"generation"}).AddRow(1))
	postRecovery, err := GenerateTokens(ctx, rdb, "user-integration", "", "user")
	if err != nil {
		t.Fatal(err)
	}
	hgExpectSessionAuthority(mock, "user-integration", 1, false)
	if after, err := RefreshToken(ctx, rdb, postRecovery.RefreshToken); err != nil || after.RefreshToken != postRecovery.RefreshToken {
		t.Fatalf("重新登录后应可按不轮换规则续签: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

type hgRegistrationSender func(context.Context, string, string) error

func (send hgRegistrationSender) SendCode(ctx context.Context, address, code string) error {
	return send(ctx, address, code)
}

func TestHGRegistrationEmailRedisIntegration(t *testing.T) {
	rdb := hgIsolatedAuthRedis(t)
	service := &UserService{redisService: PersistenceRedisPackage.HGNewRedisServiceWithClient(rdb)}
	ctx := context.Background()
	t.Run("成功发送和一次性消费", func(t *testing.T) {
		var code string
		err := service.HGSendRegistrationEmailCode(ctx, "test@EXAMPLE.ORG", hgRegistrationSender(func(_ context.Context, address, value string) error {
			if address != "test@example.org" || !Mail.HGValidCode(value) {
				t.Fatal("邮箱规范化或验证码格式不正确")
			}
			if service.VerifyEmailCode(ctx, address, value) == nil {
				t.Fatal("SMTP 未确认投递时不能使用验证码")
			}
			code = value
			return nil
		}))
		if err != nil {
			t.Fatal(err)
		}
		if err := service.HGSendRegistrationEmailCode(ctx, "test@example.org", nil); !errors.Is(err, Mail.HGErrSenderConfig) {
			t.Fatal("缺失发信端口应失败")
		}
		if err := service.HGSendRegistrationEmailCode(ctx, "test@example.org", hgRegistrationSender(func(context.Context, string, string) error {
			t.Fatal("同一邮箱在一分钟内不得重复发送")
			return nil
		})); !errors.Is(err, Mail.HGErrRateLimited) {
			t.Fatal("未执行发信限流")
		}
		if service.VerifyEmailCode(ctx, "other@example.org", code) == nil ||
			service.VerifyEmailCode(ctx, "test@example.org", code) != nil ||
			service.VerifyEmailCode(ctx, "test@example.org", code) == nil {
			t.Fatal("邮箱用途隔离或一次性消费不正确")
		}
	})
	t.Run("SMTP 失败不启用验证码", func(t *testing.T) {
		var code string
		err := service.HGSendRegistrationEmailCode(ctx, "failure@example.org", hgRegistrationSender(func(_ context.Context, _, value string) error {
			code = value
			return errors.New("隔离 SMTP 失败")
		}))
		if !errors.Is(err, Mail.HGErrUnavailable) || !Mail.HGValidCode(code) || service.VerifyEmailCode(ctx, "failure@example.org", code) == nil {
			t.Fatal("发信失败后不得保留可用验证码")
		}
	})
}
