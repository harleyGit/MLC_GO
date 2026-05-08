package HGHandlerPackage

import (
	HGMiddlewareGroupPackage "MLC_GO/internal/interfaces/middleware/middleware_group"
	"testing"
)

// TestBuildRouteCatalogContainsFullPaths 验证路由目录包含完整可调用 API 路径。
func TestBuildRouteCatalogContainsFullPaths(t *testing.T) {
	// 收集所有模块的路由清单
	catalog := make([]HGMiddlewareGroupPackage.HGRouteCatalogItem, 0, 16)
	catalog = append(catalog, HGMiddlewareGroupPackage.AuthRouteCatalog()...)
	catalog = append(catalog, HGMiddlewareGroupPackage.UserRouteCatalog()...)

	got := make(map[string]struct{}, len(catalog))
	for _, item := range catalog {
		got[item.Method+" "+item.Path] = struct{}{}
	}

	expected := []string{
		"POST /api/v1/auth/login",
		"POST /api/v1/auth/register",
		"GET /api/v1/auth/send_code",
		"GET /api/v1/profile/info",
		"GET /api/v1/profile/list",
		"PUT /api/v1/profile/security",
	}

	for _, route := range expected {
		if _, ok := got[route]; !ok {
			t.Fatalf("route %s not found in catalog", route)
		}
	}
}
