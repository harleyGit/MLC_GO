package iam

import (
	UserService "MLC_GO/internal/modules/user/service"
	SQL "MLC_GO/internal/pkg/mysql/queries"
	Redis "MLC_GO/internal/pkg/redis"
	"context"
	"database/sql"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
)

// hgRecoveryMySQL 仅启动全新数据目录，忽略 MySQL 配置且禁止 TCP/MySQL X 监听。
func hgRecoveryMySQL(t *testing.T) (context.Context, *sql.DB) {
	t.Helper()
	if os.Getenv("MLC_MFA_RECOVERY_MYSQL_INTEGRATION") != "1" {
		t.Skip("需显式授权启动临时 MySQL 实例")
	}
	binary, err := exec.LookPath("mysqld")
	if err != nil {
		t.Fatal("隔离测试需要 mysqld")
	}
	dir, err := os.MkdirTemp("", "hg-mfadb-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	ctx, cancel := context.WithTimeout(context.Background(), 55*time.Second)
	t.Cleanup(cancel)
	datadir, socket := filepath.Join(dir, "data"), filepath.Join(dir, "mysql.sock")
	init := exec.CommandContext(ctx, binary, "--no-defaults", "--initialize-insecure", "--datadir="+datadir, "--log-error="+filepath.Join(dir, "init.log"))
	if err := init.Run(); err != nil {
		t.Fatalf("初始化隔离 MySQL 失败: %v", err)
	}
	server := exec.Command(binary, "--no-defaults", "--skip-networking", "--mysqlx=OFF", "--datadir="+datadir,
		"--socket="+socket, "--pid-file="+filepath.Join(dir, "mysql.pid"), "--log-error="+filepath.Join(dir, "mysql.log"))
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Process.Signal(os.Interrupt); _ = server.Process.Kill(); _ = server.Wait() })

	cfg := mysql.NewConfig()
	cfg.Net, cfg.Addr, cfg.User, cfg.Loc, cfg.ParseTime, cfg.MultiStatements = "unix", socket, "root", time.UTC, true, true
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1) // 后续 USE 只作用这个专用连接，确保所有语句都在临时 schema。
	t.Cleanup(func() { _ = db.Close() })
	ready, stop := context.WithTimeout(ctx, 15*time.Second)
	defer stop()
	for {
		if err := db.PingContext(ready); err == nil {
			break
		}
		select {
		case <-ready.Done():
			t.Fatal("隔离 MySQL 未就绪")
		case <-time.After(50 * time.Millisecond):
		}
	}
	hgRecoveryRunSQLFile(t, ctx, db, "testdata/hg_recovery_mysql_schema.sql")
	return ctx, db
}

func hgRecoveryRunSQLFile(t *testing.T, ctx context.Context, db *sql.DB, path string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, string(raw)); err != nil {
		t.Fatalf("隔离 MySQL 执行 %s 失败: %v", path, err)
	}
}

func TestHGMFARecoveryMySQLIntegration(t *testing.T) {
	ctx, db := hgRecoveryMySQL(t)
	t.Setenv("SERVER_ENV", "debug")
	t.Setenv("MLC_IAM_MFA_KEY_HEX", strings.Repeat("0c", 32))
	const migration = "../../../migrations/000087_mfa_recovery."
	if err := UserService.HGCheckAuthRecoverySchema(ctx, db); err == nil {
		t.Fatal("无迁移时启动检查不应通过")
	}
	hgRecoveryRunSQLFile(t, ctx, db, migration+"up.sql")
	if err := UserService.HGCheckAuthRecoverySchema(ctx, db); err != nil {
		t.Fatal(err)
	}
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	rdb.AddHook(hgRecoveryRedisHook{})
	t.Cleanup(func() { _ = rdb.Close() })
	s := NewHGService(NewHGRepository(db).WithHGPermissionCache(Redis.HGNewRedisServiceWithClient(rdb)))
	var delivered string
	s.HGWithRecoverySender(hgRecoveryTestSender(func(_ context.Context, _, code string) error {
		delivered = code // 仅在临时内存保留，不写日志或外部邮箱。
		return nil
	}))
	oldCtx := UserService.HGContextWithSessionGeneration(ctx, 0)
	enroll := func(c context.Context) []byte {
		t.Helper()
		secret, _, err := s.HGEnrollOwnMFA(c, hgRecoveryTestUser, "fixture-password")
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
		if err != nil {
			t.Fatal(err)
		}
		return decoded
	}
	secret := enroll(oldCtx)
	step := time.Now().Unix() / 30
	if err := s.HGConfirmOwnMFA(oldCtx, hgRecoveryTestUser, hgTOTP(secret, step)); err != nil {
		t.Fatal(err)
	}
	const address = "fixture@icloud.com"
	if err := s.HGSendOwnRecoveryBindCode(oldCtx, hgRecoveryTestUser, address); err != nil {
		t.Fatal(err)
	}
	bindCode := delivered
	if err := s.HGBindOwnRecoveryEmail(oldCtx, hgRecoveryTestUser, address, bindCode, hgTOTP(secret, step)); !errors.Is(err, hgErrRecoveryFactor) {
		t.Fatalf("应拒绝已使用的 TOTP: %v", err)
	}
	if err := s.HGBindOwnRecoveryEmail(oldCtx, hgRecoveryTestUser, address, bindCode, hgTOTP(secret, step+1)); err != nil {
		t.Fatalf("回滚后仍可使用同一封邮件的验证码绑定: %v", err)
	}
	state, err := s.HGOwnRecoveryStatus(oldCtx, hgRecoveryTestUser)
	if err != nil || !state.EmailVerified || !state.MFAEnabled || state.EmailMasked != "***@icloud.com" || state.AvailableAt == nil {
		t.Fatalf("权威绑定状态不正确: %v", err)
	}
	if err := s.HGSendOwnRecoveryResetCode(oldCtx, hgRecoveryTestUser); !errors.Is(err, hgErrRecoveryCooldown) {
		t.Fatalf("数据库冷却期内不应发信: %v", err)
	}
	hgRecoveryRunSQLFile(t, ctx, db, "testdata/hg_recovery_mysql_ready.sql")
	if err := s.HGSendOwnRecoveryResetCode(oldCtx, hgRecoveryTestUser); err != nil {
		t.Fatal(err)
	}
	resetCode := delivered
	if err := s.HGResetOwnMFA(oldCtx, hgRecoveryTestUser, "wrong", resetCode); !errors.Is(err, hgErrRecoveryPassword) {
		t.Fatalf("应拒绝错误密码且保留邮件码: %v", err)
	}
	if err := s.HGResetOwnMFA(oldCtx, hgRecoveryTestUser, "fixture-password", resetCode); err != nil {
		t.Fatal(err)
	}
	assertAuthority := func(user, sid string, wantGeneration uint64, wantForced bool) {
		t.Helper()
		session, _ := hex.DecodeString(sid)
		var generation uint64
		var forced bool
		err := db.QueryRowContext(ctx, SQL.HGSelectAuthSessionAuthority, user, session, user).Scan(&generation, &forced)
		if err != nil || generation != wantGeneration || forced != wantForced {
			t.Fatalf("用户会话代际或 IM 撤销状态不正确: %v", err)
		}
	}
	assertAuthority(hgRecoveryTestUser, "00000000000000000000000000000011", 1, true)
	assertAuthority("fixture-other", "00000000000000000000000000000022", 0, false)
	if err := s.HGResetOwnMFA(oldCtx, hgRecoveryTestUser, "fixture-password", resetCode); !errors.Is(err, hgErrRecoveryChanged) {
		t.Fatalf("旧会话不能重放重置: %v", err)
	}
	newCtx := UserService.HGContextWithSessionGeneration(ctx, 1)
	state, err = s.HGOwnRecoveryStatus(newCtx, hgRecoveryTestUser)
	if err != nil || state.MFAEnabled || !state.EmailVerified {
		t.Fatalf("应仅重置 MFA，保留已验证邮箱: %v", err)
	}
	newSecret := enroll(newCtx)
	if err := s.HGConfirmOwnMFA(newCtx, hgRecoveryTestUser, hgTOTP(newSecret, time.Now().Unix()/30)); err != nil {
		t.Fatal(err)
	}
}
