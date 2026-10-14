package HGRouterPackage

import (
	"MLC_GO/internal/modules/recharge/handler"
	hgserver "MLC_GO/internal/pkg/server"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHGRechargeCatalogGuardAndMount(t *testing.T) {
	if len(HGRechargeRouteCatalog()) != 1 || HGRechargeRouteCatalog()[0].Path != "/api/v1/recharge/records" { t.Fatal("recharge catalog missing") }
	hgRoot := http.NewServeMux(); hgRoot.Handle(HGRechargeModuleBasePath+"/", http.StripPrefix(HGRechargeModuleBasePath, HGNewRechargeRouteGroup(handler.HGNewHandler(nil))))
	hgRoutes := hgRechargeRoutes(handler.HGNewHandler(nil))
	for _, hgRoute := range hgRoutes {
		var hgFound bool
		for _, hgRule := range hgserver.HGRechargeMethodRules() { if hgRule.Path == hgRoute.SubPath && hgRule.Methods[hgRoute.Method] && hgRule.NeedAuth { hgFound = true } }
		if !hgFound || !hgRoute.NeedAuth || hgRoute.Handler == nil { t.Fatalf("route/guard mismatch: %+v", hgRoute) }
		hgRequest := httptest.NewRequest(hgRoute.Method, hgRoute.FullPath, nil); hgRequest.Header.Set("X-API-Version", "v1")
		hgRecorder := httptest.NewRecorder(); hgRoot.ServeHTTP(hgRecorder, hgRequest)
		if hgRecorder.Code != http.StatusBadRequest { t.Fatalf("guard status=%d body=%s", hgRecorder.Code, hgRecorder.Body.String()) }
	}
}
