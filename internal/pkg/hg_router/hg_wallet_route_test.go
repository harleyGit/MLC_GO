package HGRouterPackage

import (
	"MLC_GO/internal/modules/wallet/handler"
	hgserver "MLC_GO/internal/pkg/server"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHGWalletCatalogAndGuard(t *testing.T) {
	hgHandler := handler.HGNewHandler(nil)
	hgRoot := http.NewServeMux()
	hgRoot.Handle(HGWalletModuleBasePath+"/", http.StripPrefix(HGWalletModuleBasePath, HGNewWalletRouteGroup(hgHandler)))
	if len(HGWalletRouteCatalog()) != 5 {
		t.Fatal("钱包目录必须完整")
	}
	for _, hgRoute := range hgWalletRoutes(hgHandler) {
		hgFound := false
		for _, hgRule := range hgserver.HGWalletMethodRules() {
			if hgRule.Path == hgRoute.SubPath && hgRule.Version == "v1" && hgRule.Methods[hgRoute.Method] && hgRule.NeedAuth {
				hgFound = true
			}
		}
		if !hgFound || !hgRoute.NeedAuth || hgRoute.Handler == nil {
			t.Fatalf("路由和Guard不一致: %+v", hgRoute)
		}
		for _, hgCase := range []struct {
			hgMethod, hgVersion string
			hgHeaders           bool
			hgStatus            int
		}{
			{hgRoute.Method, "v1", false, 400}, {hgRoute.Method, "v1", true, 401}, {http.MethodDelete, "v1", false, 405}, {hgRoute.Method, "v999", false, 404},
		} {
			hgRequest := httptest.NewRequest(hgCase.hgMethod, hgRoute.FullPath, nil)
			hgRequest.Header.Set("X-API-Version", hgCase.hgVersion)
			if hgCase.hgHeaders {
				for _, hgHeader := range []string{"Content-Type", "X-Device-ID", "X-Client-Type", "X-Client-Version", "X-Language", "X-Request-ID", "X-Timestamp", "X-Signature"} {
					hgRequest.Header.Set(hgHeader, "test-placeholder")
				}
			}
			hgRecorder := httptest.NewRecorder()
			hgRoot.ServeHTTP(hgRecorder, hgRequest)
			if hgRecorder.Code != hgCase.hgStatus {
				t.Fatalf("%s status=%d body=%s", hgRoute.FullPath, hgRecorder.Code, hgRecorder.Body.String())
			}
		}
	}
}
