package HGRouterPackage

import (
	UserJWTMiddlewarePackage "MLC_GO/internal/modules/user/middleware"
	UserServicePackage "MLC_GO/internal/modules/user/service"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// hgUserTargetAuthorizer 只记录跨用户授权调用，不依赖真实 IAM 服务。
type hgUserTargetAuthorizer struct {
	allowed bool
	err     error
	calls   int
}

func (stub *hgUserTargetAuthorizer) HasAssetPermission(_ context.Context, actor, permission string) (bool, error) {
	stub.calls++
	if actor != "actor" || permission != "user:directory:write" {
		return false, errors.New("错误的授权主体或权限")
	}
	return stub.allowed, stub.err
}

func TestProfileTargetWriteGuard(t *testing.T) {
	for _, tc := range []struct {
		name       string
		target     string
		authorizer *hgUserTargetAuthorizer
		wantStatus int
		wantCalls  int
	}{
		{name: "本人", target: "actor", wantStatus: http.StatusNoContent},
		{name: "缺省本人", wantStatus: http.StatusNoContent},
		{name: "他人无授权器", target: "target", wantStatus: http.StatusServiceUnavailable},
		{name: "他人无权限", target: "target", authorizer: &hgUserTargetAuthorizer{}, wantStatus: http.StatusForbidden, wantCalls: 1},
		{name: "授权器失败", target: "target", authorizer: &hgUserTargetAuthorizer{err: errors.New("iam unavailable")}, wantStatus: http.StatusServiceUnavailable, wantCalls: 1},
		{name: "他人有权限", target: "target", authorizer: &hgUserTargetAuthorizer{allowed: true}, wantStatus: http.StatusNoContent, wantCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			var authorizer HGRouteAuthorizer
			if tc.authorizer != nil {
				authorizer = tc.authorizer
			}
			guard := hgProfileTargetWriteGuard(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusNoContent)
			}, authorizer)
			req := httptest.NewRequest(http.MethodPut, "/update?user_id="+tc.target, nil)
			req = req.WithContext(context.WithValue(req.Context(), UserJWTMiddlewarePackage.UserIDKey, &UserServicePackage.HGClaims{UserID: "actor"}))
			rec := httptest.NewRecorder()
			guard(rec, req)
			if rec.Code != tc.wantStatus || (called != (tc.wantStatus == http.StatusNoContent)) {
				t.Fatalf("status=%d called=%v, want %d", rec.Code, called, tc.wantStatus)
			}
			if tc.authorizer != nil && tc.authorizer.calls != tc.wantCalls {
				t.Fatalf("授权器调用 %d 次，期望 %d 次", tc.authorizer.calls, tc.wantCalls)
			}
		})
	}
}

func TestOwnProfileOnlyRejectsOtherUser(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/account?user_id=target", nil)
	req = req.WithContext(context.WithValue(req.Context(), UserJWTMiddlewarePackage.UserIDKey, &UserServicePackage.HGClaims{UserID: "actor"}))
	rec := httptest.NewRecorder()
	hgOwnProfileOnly(func(http.ResponseWriter, *http.Request) { t.Fatal("不得读取其他用户的账号安全信息") })(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("其他用户账号安全信息应返回 403，得到 %d", rec.Code)
	}
}
