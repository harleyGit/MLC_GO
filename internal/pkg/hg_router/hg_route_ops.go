package HGRouterPackage

import (
	OpsHandlerPackage "MLC_GO/internal/modules/ops/handler"
	"net/http"
)

// opsRoutes 返回 ops 模块完整路由定义。
// 运维模块接口增长较快，单独放在本文件，避免主路由组文件堆积过多 NewRouteSpec 配置。
func opsRoutes(opsHandler *OpsHandlerPackage.Handler) []RouteSpec {
	if opsHandler == nil {
		return []RouteSpec{
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/roles", true, "创建角色", nil),
			NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/roles/list", true, "获取角色列表", nil),
			NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/users/search", true, "搜索管理员用户", nil),
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/users/roles", true, "分配用户角色", nil),
			NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/users/roles/list", true, "获取用户角色", nil),
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/menus", true, "创建菜单", nil),
			NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/menus/list", true, "获取菜单列表", nil),
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/roles/permissions", true, "分配角色权限", nil),
			NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/roles/permissions/list", true, "获取角色权限", nil),
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/files/upload", true, "上传文件", nil),
			NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/files/list", true, "获取文件列表", nil),
		}
	}

	return []RouteSpec{
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/roles", true, "创建角色", opsHandler.CreateRole),
		NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/roles/list", true, "获取角色列表", opsHandler.GetRoleList),
		NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/users/search", true, "搜索管理员用户", opsHandler.SearchAdminUsers),
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/users/roles", true, "分配用户角色", opsHandler.AssignUserRoles),
		NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/users/roles/list", true, "获取用户角色", opsHandler.GetUserRoles),
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/menus", true, "创建菜单", opsHandler.CreateMenu),
		NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/menus/list", true, "获取菜单列表", opsHandler.GetMenuList),
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/roles/permissions", true, "分配角色权限", opsHandler.AssignRolePermissions),
		NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/roles/permissions/list", true, "获取角色权限", opsHandler.GetRolePermissions),
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/files/upload", true, "上传文件", opsHandler.UploadFile),
		NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/files/list", true, "获取文件列表", opsHandler.GetFileList),
	}
}
