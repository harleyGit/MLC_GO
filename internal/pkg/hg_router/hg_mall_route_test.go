package HGRouterPackage

import (
	hgserver "MLC_GO/internal/pkg/server"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHGMallCatalogAndGuardAgree(t *testing.T) {
	items := HGMallRouteCatalog()
	rules := hgserver.HGMallMethodRules()
	if len(items) != len(rules) || len(items) != 24 {
		t.Fatalf("商城路由数=%d 规则数=%d", len(items), len(rules))
	}
	for _, item := range items {
		found := false
		for _, rule := range rules {
			if item.Path == HGMallModuleBasePath+rule.Path && rule.Methods[item.Method] && item.NeedAuth && rule.NeedAuth {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("路由缺少鉴权 Guard：%s %s", item.Method, item.Path)
		}
	}
	// 与生产入口一致使用 StripPrefix，请求应先被 Guard/JWT 拦截而不是路由 404。
	group := HGNewMallRouteGroup(nil)
	for _, route := range []struct{ method, path string }{{http.MethodGet, "/orders"}, {http.MethodGet, "/orders/pending"}, {http.MethodPost, "/orders/pay"}} {
		request := httptest.NewRequest(route.method, HGMallModuleBasePath+route.path, nil)
		request.URL.Path = strings.TrimPrefix(request.URL.Path, HGMallModuleBasePath)
		writer := httptest.NewRecorder()
		group.ServeHTTP(writer, request)
		if writer.Code == http.StatusNotFound || writer.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("已注册订单路由不可达或未禁用私密数据缓存：%s %s %d", route.method, route.path, writer.Code)
		}
	}
}
