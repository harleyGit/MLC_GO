package HGHandlerPackage

import "testing"

// TestBuildRouteCatalogContainsFullPaths 验证路由目录包含完整可调用 API 路径。
func TestBuildRouteCatalogContainsFullPaths(t *testing.T) {
	catalog := buildRouteCatalog()
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
		"GET /api/v1/test/error",
		"GET /api/v1/test/ok",
		"GET /api/v1/routes",
	}

	for _, route := range expected {
		if _, ok := got[route]; !ok {
			t.Fatalf("route %s not found in catalog", route)
		}
	}
}
