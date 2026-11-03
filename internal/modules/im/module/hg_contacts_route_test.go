package module

import (
	Contacts "MLC_GO/internal/modules/im_contacts"
	"net/http"
	"net/http/httptest"
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
