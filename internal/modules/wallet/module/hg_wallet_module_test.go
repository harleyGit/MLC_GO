package module

import (
	hgroot "MLC_GO/internal/handler"
	"MLC_GO/internal/modules/wallet/handler"
	hgrouter "MLC_GO/internal/pkg/hg_router"
	"net/http/httptest"
	"testing"
)

func TestHGWalletRootMount(t *testing.T) {
	hgPrevious := hgroot.GetRegisteredModules()
	hgroot.ClearModules()
	defer func() { hgroot.ClearModules(); hgroot.RegisterModule(hgPrevious...) }()
	hgroot.RegisterModule(&HGModule{hgHandler: handler.HGNewHandler(nil)})
	hgRoot := hgroot.NewBusinessRootHandler(hgrouter.HGWalletRouteCatalog())
	for _, hgRoute := range hgrouter.HGWalletRouteCatalog() {
		hgRecorder := httptest.NewRecorder()
		hgRoot.ServeHTTP(hgRecorder, httptest.NewRequest(hgRoute.Method, hgRoute.Path, nil))
		if hgRecorder.Code != 400 {
			t.Fatalf("根路由没有进入Guard: %s %d", hgRoute.Path, hgRecorder.Code)
		}
	}
}
