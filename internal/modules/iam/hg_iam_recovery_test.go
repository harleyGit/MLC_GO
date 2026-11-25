package iam

import (
	UserJWT "MLC_GO/internal/modules/user/middleware"
	UserService "MLC_GO/internal/modules/user/service"
	Mail "MLC_GO/internal/pkg/email"
	SQL "MLC_GO/internal/pkg/mysql/queries"
	Redis "MLC_GO/internal/pkg/redis"
	Utils "MLC_GO/internal/pkg/utils"
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
)

const hgRecoveryTestUser = "fixture-user"

type hgRecoveryRedisHook struct{ denied bool }

func (hgRecoveryRedisHook) DialHook(next redis.DialHook) redis.DialHook { return next }
func (hgRecoveryRedisHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}
func (h hgRecoveryRedisHook) ProcessHook(_ redis.ProcessHook) redis.ProcessHook {
	return func(_ context.Context, cmd redis.Cmder) error {
		switch cmd.Name() {
		case "set":
			cmd.(*redis.BoolCmd).SetVal(!h.denied)
		case "eval":
			cmd.(*redis.Cmd).SetVal(int64(1))
		default:
			return errors.New("未预期的 Redis 命令")
		}
		return nil
	}
}

type hgRecoveryTestSender func(context.Context, string, string) error

func (f hgRecoveryTestSender) SendCode(ctx context.Context, address, code string) error {
	return f(ctx, address, code)
}

func hgRecoveryFixture(t *testing.T, limited bool) (*HGService, sqlmock.Sqlmock) {
	t.Helper()
	t.Setenv("SERVER_ENV", "debug")
	t.Setenv("MLC_IAM_MFA_KEY_HEX", strings.Repeat("0c", 32))
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Error(err)
		}
		_ = db.Close()
	})
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	rdb.AddHook(hgRecoveryRedisHook{denied: limited})
	t.Cleanup(func() { _ = rdb.Close() })
	repo := NewHGRepository(db).WithHGPermissionCache(Redis.HGNewRedisServiceWithClient(rdb))
	return NewHGService(repo), mock
}

func hgExpectRecoveryLogin(mock sqlmock.Sqlmock, password string, generation uint64) {
	mock.ExpectBegin()
	mock.ExpectQuery(SQL.HGLockRecoveryAccount).WithArgs(hgRecoveryTestUser).
		WillReturnRows(sqlmock.NewRows([]string{"hash", "salt"}).AddRow(Utils.HashPassword(password, "fixture-salt"), "fixture-salt"))
	mock.ExpectQuery(SQL.HGSelectRecoveryGeneration).WithArgs(hgRecoveryTestUser).
		WillReturnRows(sqlmock.NewRows([]string{"generation"}).AddRow(generation))
}

func TestHGMFARecoverySendUsesVerifiedAddress(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		ready, limited, failed bool
		want                   error
	}{
		{name: "确认投递后启用验证码", ready: true},
		{name: "冷却中不发信", want: hgErrRecoveryCooldown},
		{name: "限流不发信", ready: true, limited: true, want: Mail.HGErrRateLimited},
		{name: "SMTP 失败不启用验证码", ready: true, failed: true, want: Mail.HGErrUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service, mock := hgRecoveryFixture(t, tc.limited)
			email := hgRecoveryEmail{address: "verified@example.org", available: time.Unix(100, 0), ready: tc.ready}
			mock.ExpectQuery(SQL.HGSelectIAMMFAEnabled).WithArgs(hgRecoveryTestUser).
				WillReturnRows(sqlmock.NewRows([]string{"enabled"}).AddRow(true))
			mock.ExpectQuery(SQL.HGSelectRecoveryEmail).WithArgs(hgRecoveryTestUser).
				WillReturnRows(sqlmock.NewRows([]string{"email", "available", "ready"}).AddRow(email.address, email.available, email.ready))
			var sent bool
			service.HGWithRecoverySender(hgRecoveryTestSender(func(_ context.Context, to, code string) error {
				sent = true
				if to != email.address || !Mail.HGValidCode(code) {
					t.Fatal("未向权威恢复邮箱发送有效格式的验证码")
				}
				if tc.failed {
					return errors.New("供应商不可用")
				}
				digest, err := hgRecoveryDigest(hgRecoveryTestUser, 4, hgRecoveryReset, email, code)
				if err != nil {
					t.Fatal(err)
				}
				mock.ExpectExec(SQL.HGSaveRecoveryChallenge).WithArgs(hgRecoveryTestUser, hgRecoveryReset, digest).WillReturnResult(sqlmock.NewResult(0, 1))
				return nil
			}))
			ctx := UserService.HGContextWithSessionGeneration(context.Background(), 4)
			if err := service.HGSendOwnRecoveryResetCode(ctx, hgRecoveryTestUser); !errors.Is(err, tc.want) || sent != (tc.ready && !tc.limited) {
				t.Fatalf("发信或错误不符合预期: %v", err)
			}
		})
	}
}

func TestHGMFARecoveryBindConsumesBothFactorsAtomically(t *testing.T) {
	for _, tc := range []struct {
		name string
		want error
	}{
		{"验证并记录已验证状态", nil},
		{"邮件码过期拒绝", hgErrRecoveryCode},
		{"MFA 不匹配时回滚邮件码", hgErrRecoveryFactor},
		{"邮箱已占用不修改其他账号", hgErrRecoveryTaken},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SERVER_ENV", "debug")
			service, mock := hgRecoveryFixture(t, false)
			ctx := UserService.HGContextWithSessionGeneration(context.Background(), 0)
			secret := []byte("local-totp-fixture")
			sealed, err := hgEncryptMFA(secret)
			if err != nil {
				t.Fatal(err)
			}
			step := time.Now().Unix() / 30
			email := hgRecoveryEmail{address: "new@example.org"}
			digest, _ := hgRecoveryDigest(hgRecoveryTestUser, 0, hgRecoveryBind, email, "012345")
			hgExpectRecoveryLogin(mock, "", 0)
			matched := int64(1)
			if tc.want == hgErrRecoveryCode {
				matched = 0
			}
			mock.ExpectExec(SQL.HGConsumeRecoveryChallenge).WithArgs(hgRecoveryTestUser, hgRecoveryBind, digest).WillReturnResult(sqlmock.NewResult(0, matched))
			if matched > 0 {
				last := int64(-1)
				if tc.want == hgErrRecoveryFactor {
					last = step // 模拟已消费时间步；邮件码必须随事务回滚。
				}
				mock.ExpectQuery(SQL.HGSelectIAMMFAFactor).WithArgs(hgRecoveryTestUser).
					WillReturnRows(sqlmock.NewRows([]string{"cipher", "enabled", "counter"}).AddRow(sealed, true, last))
			}
			if tc.want == nil || tc.want == hgErrRecoveryTaken {
				mock.ExpectExec(SQL.HGConsumeIAMMFACounter).WithArgs(step, hgRecoveryTestUser, step).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectQuery(SQL.HGSelectRecoveryEmailForUpdate).WithArgs(hgRecoveryTestUser).WillReturnError(sql.ErrNoRows)
				insert := mock.ExpectExec(SQL.HGInsertRecoveryEmail).WithArgs(hgRecoveryTestUser, email.address, 60)
				if tc.want == hgErrRecoveryTaken {
					insert.WillReturnError(&mysql.MySQLError{Number: 1062})
				} else {
					insert.WillReturnResult(sqlmock.NewResult(0, 1))
					mock.ExpectExec(SQL.HGClearRecoveryChallenges).WithArgs(hgRecoveryTestUser).WillReturnResult(sqlmock.NewResult(0, 0))
					mock.ExpectExec(SQL.HGInsertIAMAudit).WithArgs(hgRecoveryTestUser, "iam.mfa.recovery.bind", "user", hgRecoveryTestUser, "recovery-bind", []byte("{}")).WillReturnResult(sqlmock.NewResult(0, 1))
				}
			}
			if tc.want == nil {
				mock.ExpectCommit()
			} else {
				mock.ExpectRollback()
			}
			if err := service.HGBindOwnRecoveryEmail(ctx, hgRecoveryTestUser, email.address, "012345", hgTOTP(secret, step)); !errors.Is(err, tc.want) {
				t.Fatalf("绑定结果不符合预期: %v", err)
			}
		})
	}
}

func TestHGMFARecoveryResetConfirmsPasswordAndRevokes(t *testing.T) {
	for _, tc := range []struct {
		name       string
		password   string
		generation uint64
		want       error
	}{
		{"已确认密码和邮件码后下线", "fixture-password", 0, nil},
		{"密码错误不消费邮件码", "wrong", 0, hgErrRecoveryPassword},
		{"旧会话在写锁后再次拒绝", "fixture-password", 1, hgErrRecoveryChanged},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service, mock := hgRecoveryFixture(t, false)
			ctx := UserService.HGContextWithSessionGeneration(context.Background(), 0)
			email := hgRecoveryEmail{address: "verified@example.org", available: time.Unix(100, 0), ready: true}
			hgExpectRecoveryLogin(mock, "fixture-password", tc.generation)
			if tc.want != nil {
				mock.ExpectRollback()
			} else {
				digest, _ := hgRecoveryDigest(hgRecoveryTestUser, 0, hgRecoveryReset, email, "012345")
				mock.ExpectQuery(SQL.HGSelectRecoveryEmailForUpdate).WithArgs(hgRecoveryTestUser).
					WillReturnRows(sqlmock.NewRows([]string{"email", "available", "ready"}).AddRow(email.address, email.available, true))
				mock.ExpectExec(SQL.HGConsumeRecoveryChallenge).WithArgs(hgRecoveryTestUser, hgRecoveryReset, digest).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec(SQL.HGResetRecoveryMFA).WithArgs(hgRecoveryTestUser).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec(SQL.HGRevokeRecoverySessions).WithArgs(hgRecoveryTestUser).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec(SQL.HGRevokeRecoveryIMDevices).WithArgs(hgRecoveryTestUser).WillReturnResult(sqlmock.NewResult(0, 5))
				mock.ExpectExec(SQL.HGClearRecoveryChallenges).WithArgs(hgRecoveryTestUser).WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectExec(SQL.HGInsertIAMAudit).WithArgs(hgRecoveryTestUser, "iam.mfa.recovery.reset", "user", hgRecoveryTestUser, "recovery-reset", []byte("{}")).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			}
			if err := service.HGResetOwnMFA(ctx, hgRecoveryTestUser, tc.password, "012345"); !errors.Is(err, tc.want) {
				t.Fatalf("重置结果不符合预期: %v", err)
			}
		})
	}
}

func TestHGMFARecoveryHandlerRejectsOtherAccountsAndRecipient(t *testing.T) {
	handler := NewHGHandler(nil)
	for _, tc := range []struct {
		body     string
		signedIn bool
		status   int
	}{
		{`{}`, false, http.StatusUnauthorized},
		{`{"userId":"other"}`, true, http.StatusBadRequest},
		{`{"email":"other@example.org"}`, true, http.StatusBadRequest},
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/me/mfa/recovery/send-reset", strings.NewReader(tc.body))
		if tc.signedIn {
			req = req.WithContext(context.WithValue(req.Context(), UserJWT.UserIDKey, &UserService.HGClaims{UserID: hgRecoveryTestUser}))
		}
		out := httptest.NewRecorder()
		handler.HGOwnRecoverySendResetCode(out, req)
		if out.Code != tc.status || strings.Contains(out.Body.String(), "other@example.org") {
			t.Fatalf("恢复接口身份拒绝不符合预期: %d", out.Code)
		}
	}
}

func TestHGRecoveryCooldownByEnvironment(t *testing.T) {
	for _, env := range []string{"debug", "pre", "prod", "release"} {
		t.Setenv("SERVER_ENV", env)
		if got := hgRecoveryCooldown(); env == "debug" && got != 60 || env != "debug" && got != 3600 {
			t.Fatalf("%s 下恢复冷却期错误: %d", env, got)
		}
	}
	t.Setenv("SERVER_ENV", "pre")
	t.Setenv("MLC_MFA_RECOVERY_RESET_ENABLED", "")
	if hgRecoveryResetEnabled() {
		t.Fatal("混合版本发布时不得默认启用重置")
	}
	t.Setenv("MLC_MFA_RECOVERY_RESET_ENABLED", "1")
	if !hgRecoveryResetEnabled() {
		t.Fatal("实例全部支持会话代际后可显式启用")
	}
}
