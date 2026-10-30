package module

import (
	IAMPackage "MLC_GO/internal/modules/iam"
	"net/http"
	"net/http/httptest"
	"testing"
)

// IAM 路由必须由路由声明生成 Guard，缺少公共请求头时应先被 Guard 拦截。
func TestHGIAMRoutesMatchGuard(t *testing.T) {
	module := &HGModule{handler: IAMPackage.NewHGHandler(nil)}
	group := module.Handler()
	for _, spec := range hgRoutes(nil) {
		if !spec.NeedAuth {
			t.Fatalf("IAM 路由必须认证: %s %s", spec.Method, spec.SubPath)
		}
		request := httptest.NewRequest(spec.Method, spec.SubPath, nil)
		request.Header.Set("X-API-Version", "v1")
		response := httptest.NewRecorder()
		group.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("接口 Guard 未拦截: %s %s: %d", spec.Method, spec.SubPath, response.Code)
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
