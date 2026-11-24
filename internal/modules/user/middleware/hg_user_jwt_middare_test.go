package UserJWTMiddlewarePackage

import (
	UserServicePackage "MLC_GO/internal/modules/user/service"
	PkGDevicePackage "MLC_GO/internal/pkg/device"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestValidateAccessClaims_AllowsSameDeviceWithDifferentRemotePort(t *testing.T) {
	loginReq := httptest.NewRequest("GET", "http://example.com/api/v1/profile/list", nil)
	loginReq.RemoteAddr = "127.0.0.1:50001"
	loginReq.Header.Set("X-Device-ID", "web-device-001")
	loginReq.Header.Set("X-Client-Type", "web")
	loginReq.Header.Set("X-Language", "zh-CN")
	loginReq.Header.Set("User-Agent", "Mozilla/5.0 test")

	claims := &UserServicePackage.HGClaims{
		UserID: "user-1", SessionID: "abcdefabcdefabcdefabcdefabcdefab", JTI: "jti-1",
		Device: PkGDevicePackage.Fingerprint(loginReq), TokenTp: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: "mlc-go", Subject: "user-token", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}

	listReq := httptest.NewRequest("GET", "http://example.com/api/v1/profile/list", nil)
	listReq.RemoteAddr = "127.0.0.1:59999"
	listReq.Header.Set("X-Device-ID", "web-device-001")
	listReq.Header.Set("X-Client-Type", "web")
	listReq.Header.Set("X-Language", "zh-CN")
	listReq.Header.Set("User-Agent", "Mozilla/5.0 test")

	if err := validateAccessClaims(listReq, claims); err != nil {
		t.Fatalf("expected claims validation success, got error: %v", err)
	}
}

func TestValidateAccessClaims_RejectsDifferentDeviceID(t *testing.T) {
	loginReq := httptest.NewRequest("GET", "http://example.com/api/v1/profile/list", nil)
	loginReq.RemoteAddr = "127.0.0.1:50001"
	loginReq.Header.Set("X-Device-ID", "web-device-001")
	loginReq.Header.Set("X-Client-Type", "web")
	loginReq.Header.Set("X-Language", "zh-CN")
	loginReq.Header.Set("User-Agent", "Mozilla/5.0 test")

	claims := &UserServicePackage.HGClaims{
		UserID: "user-1", SessionID: "abcdefabcdefabcdefabcdefabcdefab", JTI: "jti-1",
		Device: PkGDevicePackage.Fingerprint(loginReq), TokenTp: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: "mlc-go", Subject: "user-token", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}

	listReq := httptest.NewRequest("GET", "http://example.com/api/v1/profile/list", nil)
	listReq.RemoteAddr = "127.0.0.1:59999"
	listReq.Header.Set("X-Device-ID", "web-device-XYZ")
	listReq.Header.Set("X-Client-Type", "web")
	listReq.Header.Set("X-Language", "zh-CN")
	listReq.Header.Set("User-Agent", "Mozilla/5.0 test")

	err := validateAccessClaims(listReq, claims)
	if err == nil {
		t.Fatalf("expected device fingerprint mismatch error, got nil")
	}
	if !strings.Contains(err.Error(), "device fingerprint mismatch") {
		t.Fatalf("expected mismatch error, got: %v", err)
	}
}

func TestValidateAccessClaimsRejectsLegacySessionlessAccess(t *testing.T) {
	request := httptest.NewRequest("GET", "/api/v1/profile/list", nil)
	claims := &UserServicePackage.HGClaims{
		UserID: "user-1", JTI: "jti-1", TokenTp: "access",
		RegisteredClaims: jwt.RegisteredClaims{Issuer: "mlc-go", Subject: "user-token", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	}
	if err := validateAccessClaims(request, claims); err == nil {
		t.Fatal("无会话 ID 的旧令牌不得绕过服务端淘汰与撤销")
	}
	claims.SessionID = "abcdefabcdefabcdefabcdefabcdefab"
	claims.RegisteredClaims.ExpiresAt = nil
	if err := validateAccessClaims(request, claims); err == nil {
		t.Fatal("无期限的 access JWT 不得进入业务接口")
	}
}
