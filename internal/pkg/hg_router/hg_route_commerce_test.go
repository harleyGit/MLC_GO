package HGRouterPackage

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHGCommerceCatalogGuardAndRootPrefix(t *testing.T) {
	items := HGCommerceRouteCatalog()
	rules := hgRouteGuardRules(hgCommerceRoutes(nil))
	// GET/POST /merchant/products 共用一条 Guard 方法规则。
	if len(items) != 18 || len(rules) != 17 {
		t.Fatalf("生产商品路由或 Guard 数量不一致：%d %d", len(items), len(rules))
	}
	for _, item := range items {
		found := false
		for _, rule := range rules {
			if item.Path == HGCommerceBasePath+rule.Path && rule.Methods[item.Method] && item.NeedAuth && rule.NeedAuth {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("接口缺少 JWT/API Guard：%s %s", item.Method, item.Path)
		}
		if strings.Contains(item.Path, "/pay") || strings.Contains(item.Path, "/orders") {
			t.Errorf("未接库存订单时不可开放付款路由：%s", item.Path)
		}
	}
	group := HGNewCommerceRouteGroup(nil, nil)
	for _, route := range []struct{ method, path string }{{http.MethodGet, "/products"}, {http.MethodGet, "/admin/products/detail"}, {http.MethodPost, "/merchant/products/submit"}, {http.MethodPost, "/merchant/images/delete"}} {
		request := httptest.NewRequest(route.method, HGCommerceBasePath+route.path, nil)
		request.URL.Path = strings.TrimPrefix(request.URL.Path, HGCommerceBasePath)
		writer := httptest.NewRecorder()
		group.ServeHTTP(writer, request)
		if writer.Code == http.StatusNotFound || writer.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("根入口 StripPrefix 后路由不可达：%s %s %d", route.method, route.path, writer.Code)
		}
	}
}
