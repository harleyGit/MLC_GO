package HGMiddlewareGroupPackage

import (
	UserHandlerPackage "MLC_GO/internal/modules/user/handler"
	UserServicePackage "MLC_GO/internal/modules/user/service"
	PkGDevicePackage "MLC_GO/internal/pkg/device"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TestUserMiddlewareGoup_ListMethodGuard 验证 /list 只允许 GET。
// 这里不依赖真实数据库或 Redis，因为 MethodGuard 会先于业务 handler 生效。
func TestUserMiddlewareGoup_ListMethodGuard(t *testing.T) {
	handler := UserMiddlewareGoup(&UserHandlerPackage.UserHandler{})

	req := httptest.NewRequest(http.MethodPost, "/list", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer mock-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusMethodNotAllowed, rec.Code, rec.Body.String())
	}
}

// TestUserMiddlewareGoup_ListRequireBearerToken 验证 /list 已经过 JWT 鉴权中间件。
// 这里传入一个假的 Bearer token，让请求通过 MethodGuard 和 Header 检查，
// 最终由 JWT 中间件返回 401，这样可以确认路由和鉴权链路都已接通。
func TestUserMiddlewareGoup_ListRequireBearerToken(t *testing.T) {
	handler := UserMiddlewareGoup(&UserHandlerPackage.UserHandler{})

	req := httptest.NewRequest(http.MethodGet, "/list?cursor=0&pageSize=20", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Device-ID", "device-test-001")
	req.Header.Set("X-Client-Type", "ios")
	req.Header.Set("X-Client-Version", "1.0.0")
	req.Header.Set("X-API-Version", "v1")
	req.Header.Set("X-Language", "zh-CN")
	req.Header.Set("Accept-Language", "zh-CN")
	req.Header.Set("X-Request-ID", "req-test-001")
	req.Header.Set("X-Timestamp", strconv.FormatInt(time.Now().Unix(), 10))
	req.Header.Set("User-Agent", "middleware-test")
	req.Header.Set("Authorization", buildExpiredBearerToken(req))
	req.Header.Set("X-Signature", buildSignature(req))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusUnauthorized, rec.Code, rec.Body.String())
	}
}

func buildExpiredBearerToken(r *http.Request) string {
	now := time.Now().UTC()
	claims := &UserServicePackage.HGClaims{
		UserID:  "1",
		Device:  PkGDevicePackage.Fingerprint(r),
		JTI:     "test-jti",
		TokenTp: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(-1 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now.Add(-2 * time.Minute)),
			NotBefore: jwt.NewNumericDate(now.Add(-2 * time.Minute)),
			Issuer:    "mlc-go",
			Subject:   "user-token",
		},
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(UserServicePackage.Secret)
	if err != nil {
		panic(err)
	}

	return "Bearer " + token
}

func buildSignature(r *http.Request) string {
	bodyHash := sha256.Sum256(nil)
	payload := []string{
		r.Method,
		r.URL.Path,
		r.Header.Get("X-Timestamp"),
		r.Header.Get("X-Request-ID"),
		r.Header.Get("X-Device-ID"),
		r.Header.Get("X-Client-Type"),
		r.Header.Get("X-Client-Version"),
		r.Header.Get("X-API-Version"),
		r.Header.Get("X-Language"),
		hex.EncodeToString(bodyHash[:]),
		r.Header.Get("Authorization"),
	}

	mac := hmac.New(sha256.New, UserServicePackage.Secret)
	mac.Write([]byte(strings.Join(payload, "\n")))

	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
