package HGRouterPackage

import (
	OpsHandlerPackage "MLC_GO/internal/modules/ops/handler"
	HGServerPackage "MLC_GO/internal/pkg/server"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHGRechargeSKURoutes(t *testing.T) {
	for _, handler := range []*OpsHandlerPackage.Handler{nil, OpsHandlerPackage.NewHandler(nil)} {
		want := map[string]bool{"POST /api/v1/ops/payment/recharge/skus": false, "GET /api/v1/ops/payment/recharge/skus/list": false, "POST /api/v1/ops/payment/recharge/skus/update": false, "POST /api/v1/ops/payment/recharge/skus/delete": false}
		for _, route := range opsRoutes(handler) {
			key := route.Method + " " + route.FullPath
			if _, ok := want[key]; ok {
				if !route.NeedAuth || handler != nil && route.Handler == nil {
					t.Fatalf("invalid route %+v", route)
				}
				want[key] = true
			}
		}
		for route, found := range want {
			if !found {
				t.Fatalf("missing %s", route)
			}
		}
	}
}

// TestHGOpsCatalogGuardParity 防止展示目录已有接口而独立 Guard 目录漏登记。
func TestHGOpsCatalogGuardParity(t *testing.T) {
	for _, hgRoute := range opsRoutes(OpsHandlerPackage.NewHandler(nil)) {
		hgFound := false
		for _, hgRule := range HGServerPackage.OpsMethodRules() {
			if hgRule.Path == hgRoute.SubPath && hgRule.Version == "v1" && hgRule.Methods[hgRoute.Method] && hgRule.NeedAuth == hgRoute.NeedAuth {
				hgFound = true
			}
		}
		if !hgFound {
			t.Errorf("Guard rule missing or inconsistent: %s %s", hgRoute.Method, hgRoute.FullPath)
		}
	}
}

// TestHGRechargeSKUGuardChain 使用真实路由组及根入口相同的 StripPrefix，不启动监听或访问数据库。
func TestHGRechargeSKUGuardChain(t *testing.T) {
	hgHandler := OpsHandlerPackage.NewHandler(nil)
	hgRoot := http.NewServeMux()
	hgRoot.Handle(OpsModuleBasePath+"/", http.StripPrefix(OpsModuleBasePath, NewOpsRouteGroup(hgHandler)))
	hgMux := http.NewServeMux()
	BindRouteSpecs(hgMux, opsRoutes(hgHandler))
	for _, hgRoute := range opsRoutes(hgHandler) {
		if !strings.HasPrefix(hgRoute.SubPath, "/payment/recharge/skus") {
			continue
		}
		t.Run(hgRoute.SubPath, func(t *testing.T) {
			_, hgPattern := hgMux.Handler(httptest.NewRequest(hgRoute.Method, hgRoute.SubPath, nil))
			if hgPattern != hgRoute.SubPath {
				t.Fatalf("business mux binding = %q", hgPattern)
			}
			for _, hgCase := range []struct {
				hgName, hgMethod, hgVersion string
				hgHeaders                   bool
				hgStatus                    int
				hgMessage                   string
			}{
				{"missing headers", hgRoute.Method, "v1", false, http.StatusBadRequest, "100004"},
				{"missing authorization", hgRoute.Method, "v1", true, http.StatusUnauthorized, "Authorization不能为空"},
				{"wrong method", http.MethodDelete, "v1", false, http.StatusMethodNotAllowed, "method not allowed"},
				{"unknown version", hgRoute.Method, "v999", false, http.StatusNotFound, "interface not found"},
			} {
				t.Run(hgCase.hgName, func(t *testing.T) {
					hgReq := httptest.NewRequest(hgCase.hgMethod, hgRoute.FullPath+"?cursor=&pageSize=20", nil)
					if hgCase.hgHeaders {
						for _, hgHeader := range []string{"Content-Type", "X-Device-ID", "X-Client-Type", "X-Client-Version", "X-Language", "X-Request-ID", "X-Timestamp", "X-Signature"} {
							hgReq.Header.Set(hgHeader, "test-placeholder")
						}
					}
					hgReq.Header.Set("X-API-Version", hgCase.hgVersion)
					hgRec := httptest.NewRecorder()
					hgRoot.ServeHTTP(hgRec, hgReq)
					if hgRec.Code != hgCase.hgStatus || !strings.Contains(hgRec.Body.String(), hgCase.hgMessage) {
						t.Fatalf("status=%d body=%s", hgRec.Code, hgRec.Body.String())
					}
				})
			}
		})
	}
}
