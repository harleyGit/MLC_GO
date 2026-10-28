package module

import (
	IAMPackage "MLC_GO/internal/modules/iam"
	HGServerPackage "MLC_GO/internal/pkg/server"
	"net/http"
	"net/http/httptest"
	"testing"
)

// 路由和 API Guard 一一对应，避免新增页面接口在 Guard 层变成 404。
func TestHGIAMRoutesMatchGuard(t *testing.T) {
	specs := hgRoutes(nil)
	rules := HGServerPackage.IAMMethodRules()
	if len(specs) != len(rules) {
		t.Fatalf("IAM 路由数 %d 与 Guard 规则数 %d 不一致", len(specs), len(rules))
	}
	for _, spec := range specs {
		found := false
		for _, rule := range rules {
			if rule.Path == spec.SubPath && rule.Methods[spec.Method] && rule.NeedAuth && spec.NeedAuth {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("缺少接口 Guard 规则: %s %s", spec.Method, spec.SubPath)
		}
	}
}

// 根入口相同的 StripPrefix 链必须能把 IAM 路径送入 Guard；无 JWT 可拒绝，但不能返回路由 404。
func TestHGIAMRouteReachableThroughStripPrefix(t *testing.T) {
	module := &HGModule{handler: IAMPackage.NewHGHandler(nil)}
	root := http.NewServeMux()
	root.Handle(hgBasePath+"/", http.StripPrefix(hgBasePath, module.Handler()))
	request := httptest.NewRequest(http.MethodGet, hgBasePath+"/roles/list?cursor=0", nil)
	response := httptest.NewRecorder()
	root.ServeHTTP(response, request)
	if response.Code == http.StatusNotFound {
		t.Fatalf("IAM 列表接口被 Guard 或路由返回 404: %s", response.Body.String())
	}
}
