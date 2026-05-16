package UserJWTMiddlewarePackage

import (
	UserServicePackage "MLC_GO/internal/modules/user/service"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestAuthMiddleware_MissingTokenReturnStandardError(t *testing.T) {
	handler := AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("next handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/profile/info", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status=%d, got=%d", http.StatusUnauthorized, rec.Code)
	}

	assertStandardTokenInvalidBody(t, rec.Body.Bytes())
}

func TestAuthMiddleware_DeviceMismatchHideInternalReason(t *testing.T) {
	claims := &UserServicePackage.HGClaims{
		UserID:  "1",
		Device:  "another-device-fingerprint",
		TokenTp: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "mlc-go",
			Subject:   "user-token",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(10 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-1 * time.Minute)),
			NotBefore: jwt.NewNumericDate(time.Now().Add(-1 * time.Minute)),
		},
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(UserServicePackage.Secret)
	if err != nil {
		t.Fatalf("sign token failed: %v", err)
	}

	handler := AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("next handler should not be called")
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/profile/info", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Device-ID", "web-device-001")
	req.Header.Set("X-Client-Type", "web")
	req.Header.Set("X-Language", "zh-CN")
	req.Header.Set("User-Agent", "Mozilla/5.0 test")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status=%d, got=%d", http.StatusUnauthorized, rec.Code)
	}

	body := rec.Body.String()
	if strings.Contains(body, "device fingerprint mismatch") {
		t.Fatalf("response should not expose internal reason, body=%s", body)
	}

	assertStandardTokenInvalidBody(t, rec.Body.Bytes())
}

func assertStandardTokenInvalidBody(t *testing.T, body []byte) {
	t.Helper()

	var resp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal response failed: %v, body=%s", err, string(body))
	}

	if resp.Code != 101001 {
		t.Fatalf("expected code=101001, got=%d, body=%s", resp.Code, string(body))
	}
	if resp.Message != "Token无效" {
		t.Fatalf("expected message=Token无效, got=%s, body=%s", resp.Message, string(body))
	}
}
