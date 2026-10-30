package HGRouterPackage

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHGAddressRouteGuard 使用实际模块 StripPrefix 链检查 Guard 方法、路径和鉴权声明。
func TestHGAddressRouteGuard(t *testing.T) {
	root := http.NewServeMux()
	root.Handle(UserProfileModuleBasePath+"/", http.StripPrefix(UserProfileModuleBasePath, NewUserRouteGroup(nil)))
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/addresses"}, {http.MethodPost, "/addresses"},
		{http.MethodGet, "/addresses/detail"}, {http.MethodPut, "/addresses/detail"},
		{http.MethodDelete, "/addresses/detail"}, {http.MethodPut, "/addresses/default"},
	} {
		req := hgSignedPermissionRequest(t, tc.method, tc.path, "invalid")
		req.URL.Path = UserProfileModuleBasePath + tc.path
		resp := httptest.NewRecorder()
		root.ServeHTTP(resp, req)
		if resp.Code != http.StatusUnauthorized {
			t.Errorf("%s %s 状态=%d，期望鉴权拒绝而非路由丢失，响应=%s", tc.method, tc.path, resp.Code, resp.Body.String())
		}
	}
	req := hgSignedPermissionRequest(t, http.MethodPatch, "/addresses/detail", "invalid")
	req.URL.Path = UserProfileModuleBasePath + "/addresses/detail"
	resp := httptest.NewRecorder()
	root.ServeHTTP(resp, req)
	if resp.Code != http.StatusMethodNotAllowed {
		t.Fatalf("未开放的方法状态=%d", resp.Code)
	}
}
