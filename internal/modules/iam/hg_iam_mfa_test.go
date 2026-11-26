package iam

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

// enroll 已启用因子的专用业务码不得与普通 IAM 冲突混淆；权限/配置错误仍按原协议。
func TestHGMFAAlreadyEnabledErrorMapping(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
		code   int
	}{
		{"已启用", ErrHGMFAAlreadyEnabled, http.StatusConflict, 404008},
		{"未绑定", ErrHGMFANotEnrolled, http.StatusConflict, 404009},
		{"未启用", ErrHGMFANotEnabled, http.StatusConflict, 404010},
		{"动态码无效", ErrHGMFACodeInvalid, http.StatusForbidden, 404011},
		{"动态码已使用", ErrHGMFACodeUsed, http.StatusConflict, 404012},
		{"尝试超限", ErrHGMFARateLimited, http.StatusTooManyRequests, 404013},
		{"目标非活动管理员", ErrHGGrantTarget, http.StatusNotFound, 404014},
		{"普通冲突", ErrHGConflict, http.StatusConflict, 100009},
		{"无权", ErrHGForbidden, http.StatusForbidden, 300004},
		{"密钥不可用", ErrHGMFAKeyUnavailable, http.StatusServiceUnavailable, 500001},
		{"限流服务不可用", ErrHGMFAUnavailable, http.StatusServiceUnavailable, 500001},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			hgHTTPError(w, httptest.NewRequest(http.MethodPost, "/api/v1/iam/mfa/enroll", nil), tc.err)
			var body struct {
				Code int `json:"code"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if w.Code != tc.status || body.Code != tc.code {
				t.Fatalf("预期 HTTP %d/业务码 %d，实际 HTTP %d/业务码 %d", tc.status, tc.code, w.Code, body.Code)
			}
		})
	}
}

// MFA 写入前的因子状态、动态码校验与已使用时间步须分别返回稳定错误，拒绝时不能推进计数器。
func TestHGConsumeMFAFailureReasons(t *testing.T) {
	t.Setenv("MLC_IAM_MFA_KEY_HEX", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	secret := []byte("12345678901234567890") // RFC 6238 测试夹具，不是线上因子。
	ciphertext, err := hgEncryptMFA(secret)
	if err != nil {
		t.Fatal(err)
	}
	step := time.Now().Unix() / 30
	code := hgTOTP(secret, step)
	for _, tc := range []struct {
		name    string
		rows    *sqlmock.Rows
		code    string
		zeroRow bool
		want    error
	}{
		{"未绑定", sqlmock.NewRows([]string{"secret_cipher", "enabled", "last_counter"}), code, false, ErrHGMFANotEnrolled},
		{"未启用", sqlmock.NewRows([]string{"secret_cipher", "enabled", "last_counter"}).AddRow(ciphertext, false, -1), code, false, ErrHGMFANotEnabled},
		{"无效", sqlmock.NewRows([]string{"secret_cipher", "enabled", "last_counter"}).AddRow(ciphertext, true, -1), "invalid", false, ErrHGMFACodeInvalid},
		{"已使用", sqlmock.NewRows([]string{"secret_cipher", "enabled", "last_counter"}).AddRow(ciphertext, true, step), code, false, ErrHGMFACodeUsed},
		{"写入竞争", sqlmock.NewRows([]string{"secret_cipher", "enabled", "last_counter"}).AddRow(ciphertext, true, -1), code, true, ErrHGMFACodeUsed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service, mock := hgTestService(t)
			mock.ExpectBegin()
			mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectIAMMFAFactor)).WithArgs("ACTOR").WillReturnRows(tc.rows)
			if tc.zeroRow {
				mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGConsumeIAMMFACounter)).
					WithArgs(sqlmock.AnyArg(), "ACTOR", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 0))
			}
			mock.ExpectRollback()
			tx, beginErr := service.repo.db.BeginTx(context.Background(), nil)
			if beginErr != nil {
				t.Fatal(beginErr)
			}
			if got := service.hgConsumeMFATx(context.Background(), tx, "ACTOR", tc.code); !errors.Is(got, tc.want) {
				t.Fatalf("预期 %v，实际 %v", tc.want, got)
			}
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				t.Fatal(rollbackErr)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
	service, _ := hgTestService(t)
	if err := service.hgLimitMFA(context.Background(), "ACTOR"); !errors.Is(err, ErrHGMFAUnavailable) {
		t.Fatalf("限流依赖不可用时不应伪装成权限拒绝: %v", err)
	}
}

// 配置缺失和密码/授权拒绝保持不同状态；不向客户端输出密钥值或内部配置路径。
func TestHGMFAKeyUnavailableIsNotPermissionDenial(t *testing.T) {
	t.Setenv("MLC_IAM_MFA_KEY_HEX", "")
	if _, err := hgEncryptMFA([]byte("fixture-only")); !errors.Is(err, ErrHGMFAKeyUnavailable) {
		t.Fatalf("缺失 MFA 密钥应区别于无权限: %v", err)
	}
	t.Setenv("MLC_IAM_MFA_KEY_HEX", "invalid-format")
	if _, err := hgMFAKey(); !errors.Is(err, ErrHGMFAKeyUnavailable) {
		t.Fatalf("格式错误的 MFA 密钥应停止服务: %v", err)
	}
	for _, tc := range []struct {
		err     error
		status  int
		message string
	}{
		{ErrHGMFAKeyUnavailable, http.StatusServiceUnavailable, "MFA 服务暂不可用"},
		{ErrHGForbidden, http.StatusForbidden, "无权执行此操作"},
	} {
		w := httptest.NewRecorder()
		hgHTTPError(w, httptest.NewRequest(http.MethodPost, "/api/v1/iam/mfa/enroll", nil), tc.err)
		if w.Code != tc.status || !strings.Contains(w.Body.String(), tc.message) || strings.Contains(w.Body.String(), "MLC_IAM_MFA_KEY_HEX") {
			t.Fatalf("错误边界响应不正确: status=%d body=%s", w.Code, w.Body.String())
		}
	}
}

// RFC 6238 SHA-1 测试向量在 6 位协议下取模，覆盖时间窗与一次性消费。
func TestHGTOTPVectorAndReplay(t *testing.T) {
	secret := []byte("12345678901234567890")
	if got := hgTOTP(secret, 1); got != "287082" {
		t.Fatalf("TOTP=%s", got)
	}
	step, ok := hgVerifyTOTP(secret, "287082", -1, time.Unix(59, 0))
	if !ok || step != 1 {
		t.Fatalf("首次验证码应被接受: %d %v", step, ok)
	}
	if _, ok = hgVerifyTOTP(secret, "287082", step, time.Unix(59, 0)); ok {
		t.Fatal("已消费时间步不能重放")
	}
	if _, ok = hgVerifyTOTP(secret, "28708x", -1, time.Unix(59, 0)); ok {
		t.Fatal("非数字验证码不能通过")
	}
}

func TestHGMFAEncryption(t *testing.T) {
	t.Setenv("MLC_IAM_MFA_KEY_HEX", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	secret := []byte("12345678901234567890")
	ciphertext, err := hgEncryptMFA(secret)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ciphertext, secret) {
		t.Fatal("密文包含明文")
	}
	decoded, err := hgDecryptMFA(ciphertext)
	if err != nil || !bytes.Equal(decoded, secret) {
		t.Fatalf("解密失败: %v", err)
	}
	ciphertext[len(ciphertext)-1] ^= 1
	if _, err = hgDecryptMFA(ciphertext); err == nil {
		t.Fatal("篡改密文应被拒绝")
	}
}
