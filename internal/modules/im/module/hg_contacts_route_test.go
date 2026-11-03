package module

import (
	Contacts "MLC_GO/internal/modules/im_contacts"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 新增路由必须同时存在于 IM 目录、实际 mux 和 API Guard 规则中；缺少 JWT 不得放行。
func TestHGContactsRouteCatalogAndGuard(t *testing.T) {
	const path = "/api/v1/im/contacts/friends"
	found := false
	for _, item := range HGRouteCatalog() {
		if item.Path == path && item.Method == http.MethodGet && item.NeedAuth {
			found = true
		}
	}
	if !found {
		t.Fatal("缺少联系人鉴权路由目录")
	}
	module := &HGModule{contacts: Contacts.HGNewHandler(nil)}
	request := httptest.NewRequest(http.MethodGet, "/contacts/friends", nil)
	request.Header.Set("X-API-Version", "v1")
	response := httptest.NewRecorder()
	module.Handler().ServeHTTP(response, request)
	if response.Code == http.StatusNotFound || response.Code == http.StatusOK {
		t.Fatalf("未认证请求不应返回 %d", response.Code)
	}
}

// 敏感身份搜索必须沿用 IM 鉴权路由，不能因为新增手机号/邮箱检索而对匿名请求开放。
func TestHGContactSearchRouteRequiresAuth(t *testing.T) {
	const path = "/api/v1/im/contacts/search"
	found := false
	for _, item := range HGRouteCatalog() {
		if item.Path == path && item.Method == http.MethodPost && item.NeedAuth {
			found = true
		}
	}
	if !found {
		t.Fatal("联系人搜索缺少鉴权路由")
	}
	module := &HGModule{contacts: Contacts.HGNewHandler(nil)}
	request := httptest.NewRequest(http.MethodPost, "/contacts/search", strings.NewReader(`{"type":"phone","keyword":"13800138000"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-API-Version", "v1")
	request.Header.Set("X-Device-ID", "device-test")
	request.Header.Set("X-Client-Type", "web")
	request.Header.Set("X-Client-Version", "1")
	request.Header.Set("X-Language", "zh-CN")
	request.Header.Set("X-Request-ID", "search-test")
	request.Header.Set("X-Timestamp", "1")
	request.Header.Set("X-Signature", "invalid")
	response := httptest.NewRecorder()
	module.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("未认证搜索请求应返回 401，实际 %d", response.Code)
	}
}
