package module

import (
	IAMPackage "MLC_GO/internal/modules/iam"
	HGPermissionPackage "MLC_GO/internal/pkg/permission"
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

// 待审批授权按目标用户查询必须在路由目录和 Guard 中保持相同的 IAM 读权限。
func TestHGPendingGrantRouteCatalogAndGuard(t *testing.T) {
	var found bool
	for _, item := range RouteCatalog() {
		if item.Path == hgBasePath+"/bindings/pending" && item.Method == http.MethodGet {
			found = true
			if !item.NeedAuth || len(item.Permissions) != 1 || item.Permissions[0] != HGPermissionPackage.HGIAMRoleRead {
				t.Fatalf("目标用户待审批路由权限不一致: %+v", item)
			}
		}
	}
	if !found {
		t.Fatal("待审批查询未注册到路由目录")
	}
	root := http.NewServeMux()
	root.Handle(hgBasePath+"/", http.StripPrefix(hgBasePath, (&HGModule{handler: IAMPackage.NewHGHandler(nil)}).Handler()))
	response := httptest.NewRecorder()
	root.ServeHTTP(response, httptest.NewRequest(http.MethodGet, hgBasePath+"/bindings/pending?userId=TARGET", nil))
	if response.Code == http.StatusNotFound {
		t.Fatal("待审批查询未经过根路由 StripPrefix + Guard")
	}
}

// 即时授权路由须独立于双人审批入口，RouteSpec 与根 StripPrefix 的 Guard 权限保持一致。
func TestHGDirectGrantRouteCatalogAndGuard(t *testing.T) {
	var found bool
	for _, item := range RouteCatalog() {
		if item.Path == hgBasePath+"/bindings/grant-direct" && item.Method == http.MethodPost {
			found = true
			if !item.NeedAuth || len(item.Permissions) != 2 || item.Permissions[0] != HGPermissionPackage.HGIAMRoleGrant || item.Permissions[1] != HGPermissionPackage.HGIAMRoleGrantDirect {
				t.Fatalf("即时授权路由权限不一致: %+v", item)
			}
		}
	}
	if !found {
		t.Fatal("即时授权路由未注册")
	}
	root := http.NewServeMux()
	root.Handle(hgBasePath+"/", http.StripPrefix(hgBasePath, (&HGModule{handler: IAMPackage.NewHGHandler(nil)}).Handler()))
	response := httptest.NewRecorder()
	root.ServeHTTP(response, httptest.NewRequest(http.MethodPost, hgBasePath+"/bindings/grant-direct", nil))
	if response.Code == http.StatusNotFound {
		t.Fatal("即时授权未进入根路由的 Guard")
	}
}

// 候选资源目录仅供持有即时授权权限的管理员读取，不暴露给仅可查看角色的用户。
func TestHGScopeResourceRoutePermissions(t *testing.T) {
	found := 0
	for _, item := range RouteCatalog() {
		if (item.Path != hgBasePath+"/scope-resources/list" || item.Method != http.MethodGet) &&
			(item.Path != hgBasePath+"/scope-resources" || item.Method != http.MethodPost) &&
			(item.Path != hgBasePath+"/business-scopes" || item.Method != http.MethodPost) &&
			(item.Path != hgBasePath+"/business-scopes/disable" || item.Method != http.MethodPost) {
			continue
		}
		if !item.NeedAuth || len(item.Permissions) != 2 ||
			item.Permissions[0] != HGPermissionPackage.HGIAMRoleGrant ||
			item.Permissions[1] != HGPermissionPackage.HGIAMRoleGrantDirect {
			t.Fatalf("候选资源路由权限不正确: %+v", item)
		}
		found++
	}
	if found != 4 {
		t.Fatalf("候选资源读取、占位登记与业务实体创建/停用路由须同时注册: got %d", found)
	}
	root := http.NewServeMux()
	root.Handle(hgBasePath+"/", http.StripPrefix(hgBasePath, (&HGModule{handler: IAMPackage.NewHGHandler(nil)}).Handler()))
	response := httptest.NewRecorder()
	root.ServeHTTP(response, httptest.NewRequest(http.MethodPost, hgBasePath+"/business-scopes", nil))
	if response.Code == http.StatusNotFound {
		t.Fatal("业务实体创建接口未通过根路由 Guard")
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
