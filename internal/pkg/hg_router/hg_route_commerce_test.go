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
	// GET/POST /merchant/products 及 GET/POST /orders 各共用一条 Guard 方法规则。
	if len(items) != 27 || len(rules) != 25 {
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
		// 交易写路由虽可发现，环境与渠道均必须在处理器内 fail-closed。
	}
	group := HGNewCommerceRouteGroup(nil, nil)
	for _, route := range []struct{ method, path string }{{http.MethodGet, "/products"}, {http.MethodGet, "/admin/products/detail"}, {http.MethodPost, "/merchant/products/submit"}, {http.MethodPost, "/merchant/images/delete"}, {http.MethodPost, "/orders/payment"}} {
		request := httptest.NewRequest(route.method, HGCommerceBasePath+route.path, nil)
		request.URL.Path = strings.TrimPrefix(request.URL.Path, HGCommerceBasePath)
		writer := httptest.NewRecorder()
		group.ServeHTTP(writer, request)
		if writer.Code == http.StatusNotFound || writer.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("根入口 StripPrefix 后路由不可达：%s %s %d", route.method, route.path, writer.Code)
		}
	}
}
