package module

import (
	hgroot "MLC_GO/internal/handler"
	"MLC_GO/internal/modules/recharge/handler"
	hgrouter "MLC_GO/internal/pkg/hg_router"
	"net/http/httptest"
	"testing"
)

func TestHGRechargeRootMount(t *testing.T) {
	hgPrevious := hgroot.GetRegisteredModules()
	hgroot.ClearModules()
	defer func() { hgroot.ClearModules(); hgroot.RegisterModule(hgPrevious...) }()
	hgModule := &HGModule{hgHandler: handler.HGNewHandler(nil)}
	if hgModule.Name() != "recharge" || hgModule.BasePath() != "/api/v1/recharge" {
		t.Fatal("模块元信息错误")
	}
	hgroot.RegisterModule(hgModule)
	hgRoot := hgroot.NewBusinessRootHandler(hgrouter.HGRechargeRouteCatalog())
	hgRecorder := httptest.NewRecorder()
	hgRoot.ServeHTTP(hgRecorder, httptest.NewRequest("GET", "/api/v1/recharge/records", nil))
	if hgRecorder.Code != 400 {
		t.Fatalf("未进入签名守卫: %d", hgRecorder.Code)
	}
}
