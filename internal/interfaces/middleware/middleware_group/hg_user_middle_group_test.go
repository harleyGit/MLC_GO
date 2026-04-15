package HGMiddlewareGroupPackage

import (
	UserHandlerPackage "MLC_GO/internal/modules/user/handler"
	"net/http"
	"net/http/httptest"
	"testing"
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
	req.Header.Set("Authorization", "Bearer mock-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusUnauthorized, rec.Code, rec.Body.String())
	}
}
