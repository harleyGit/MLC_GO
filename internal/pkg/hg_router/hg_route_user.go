package HGRouterPackage

import (
	UserHandlerPackage "MLC_GO/internal/modules/user/handler"
	"net/http"
)

// userRoutes 返回 profile 模块完整路由定义。
func userRoutes(userHandler *UserHandlerPackage.HGUserHandler) []RouteSpec {
	if userHandler == nil {
		return []RouteSpec{
			NewRouteSpec("profile", http.MethodGet, UserProfileModuleBasePath, "/info", true, "获取当前用户信息", nil),
			NewRouteSpec("profile", http.MethodGet, UserProfileModuleBasePath, "/list", true, "获取用户分页列表", nil),
			NewRouteSpec("profile", http.MethodGet, UserProfileModuleBasePath, "/account", true, "获取用户账号安全信息", nil),
			NewRouteSpec("profile", http.MethodPut, UserProfileModuleBasePath, "/update", true, "更新用户资料", nil),
			NewRouteSpec("profile", http.MethodPut, UserProfileModuleBasePath, "/security", true, "更新账号安全信息", nil),
			NewRouteSpec("profile", http.MethodGet, UserProfileModuleBasePath, "/avatar", true, "头像操作（POST上传/GET获取）", nil),
		}
	}

	return []RouteSpec{
		NewRouteSpec("profile", http.MethodGet, UserProfileModuleBasePath, "/info", true, "获取当前用户信息", userHandler.Profile),
		NewRouteSpec("profile", http.MethodGet, UserProfileModuleBasePath, "/list", true, "获取用户分页列表", userHandler.GetUserList),
		NewRouteSpec("profile", http.MethodGet, UserProfileModuleBasePath, "/account", true, "获取用户账号安全信息", userHandler.GetSecurityInfo),
		NewRouteSpec("profile", http.MethodPut, UserProfileModuleBasePath, "/update", true, "更新用户资料", userHandler.UpdateProfile),
		NewRouteSpec("profile", http.MethodPut, UserProfileModuleBasePath, "/security", true, "更新账号安全信息", userHandler.UpdateSecurity),
		NewRouteSpec("profile", http.MethodGet, UserProfileModuleBasePath, "/avatar", true, "头像操作（POST上传/GET获取）", userHandler.Avatar),
	}
}
