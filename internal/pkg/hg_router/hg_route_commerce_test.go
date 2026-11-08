package HGRouterPackage

import (
	commerce "MLC_GO/internal/modules/commerce"
	permission "MLC_GO/internal/pkg/permission"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHGCommerceCatalogGuardAndRootPrefix(t *testing.T) {
	items := HGCommerceRouteCatalog()
	rules := hgRouteGuardRules(hgCommerceRoutes(nil))
	// GET/POST /merchant/products 及 GET/POST /orders 各共用一条 Guard 方法规则。
	if len(items) != 32 || len(rules) != 30 {
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

func TestHGCommercePaymentCallbackJWTGuardAndIAM(t *testing.T) {
	hgConfigureDeleteFenceTest(t)
	for _, tc := range []struct {
		name, token  string
		allowed      bool
		badSignature bool
		want         int
	}{
		{"无登录", "", false, false, http.StatusUnauthorized},
		{"伪造令牌", "invalid", false, false, http.StatusUnauthorized},
		{"无管理员权限", "valid", false, false, http.StatusForbidden},
		{"错误 API 签名", "valid", true, true, http.StatusUnauthorized},
		{"管理员通过守卫后仍受环境开关限制", "valid", true, false, http.StatusServiceUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SERVER_ENV", "prod")
			t.Setenv("MLC_COMMERCE_DEBUG_TRADING", "true")
			stub := &hgRouteAuthorizerStub{hgAllowed: map[string]bool{string(permission.HGMallMerchantReview): tc.allowed}}
			root := http.NewServeMux()
			root.Handle(HGCommerceBasePath+"/", http.StripPrefix(HGCommerceBasePath, HGNewCommerceRouteGroup(commerce.HGNewHandler(nil, nil), stub)))
			request := hgSignedPermissionRequest(t, http.MethodPost, "/admin/simulated-payment/callback", tc.token)
			request.URL.Path = HGCommerceBasePath + "/admin/simulated-payment/callback"
			if tc.badSignature {
				request.Header.Set("X-Signature", "bad")
			}
			writer := httptest.NewRecorder()
			root.ServeHTTP(writer, request)
			if writer.Code != tc.want {
				t.Fatalf("JWT/API Guard/IAM 状态=%d，期望=%d", writer.Code, tc.want)
			}
		})
	}
}
