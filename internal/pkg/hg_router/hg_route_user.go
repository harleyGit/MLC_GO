package HGRouterPackage

import (
	UserHandlerPackage "MLC_GO/internal/modules/user/handler"
	HGPermissionPackage "MLC_GO/internal/pkg/permission"
	"net/http"
)

// userRoutes 返回 profile 模块完整路由定义。
func userRoutes(userHandler *UserHandlerPackage.HGUserHandler) []RouteSpec {
	if userHandler == nil {
		return []RouteSpec{
			NewRouteSpec("profile", http.MethodGet, UserProfileModuleBasePath, "/info", true, "获取当前用户信息", nil),
			NewRouteSpec("profile", http.MethodGet, UserProfileModuleBasePath, "/list", true, "获取用户分页列表", nil, HGPermissionPackage.HGUserDirectoryRead),
			NewRouteSpec("profile", http.MethodGet, UserProfileModuleBasePath, "/account", true, "获取用户账号安全信息", nil),
			NewRouteSpec("profile", http.MethodPut, UserProfileModuleBasePath, "/update", true, "更新用户资料", nil),
			NewRouteSpec("profile", http.MethodPut, UserProfileModuleBasePath, "/security", true, "更新账号安全信息", nil),
			NewRouteSpec("profile", http.MethodGet, UserProfileModuleBasePath, "/avatar", true, "获取本人头像", nil),
			NewRouteSpec("profile", http.MethodPost, UserProfileModuleBasePath, "/avatar", true, "上传本人头像", nil),
			NewRouteSpec("profile", http.MethodGet, UserProfileModuleBasePath, "/addresses", true, "获取本人收货地址列表", nil),
			NewRouteSpec("profile", http.MethodPost, UserProfileModuleBasePath, "/addresses", true, "新增本人收货地址", nil),
			NewRouteSpec("profile", http.MethodGet, UserProfileModuleBasePath, "/addresses/detail", true, "获取本人收货地址", nil),
			NewRouteSpec("profile", http.MethodPut, UserProfileModuleBasePath, "/addresses/detail", true, "修改本人收货地址", nil),
			NewRouteSpec("profile", http.MethodDelete, UserProfileModuleBasePath, "/addresses/detail", true, "删除本人收货地址", nil),
			NewRouteSpec("profile", http.MethodPut, UserProfileModuleBasePath, "/addresses/default", true, "设置本人默认地址", nil),
		}
	}

	return []RouteSpec{
		NewRouteSpec("profile", http.MethodGet, UserProfileModuleBasePath, "/info", true, "获取当前用户信息", userHandler.Profile),
		NewRouteSpec("profile", http.MethodGet, UserProfileModuleBasePath, "/list", true, "获取用户分页列表", userHandler.GetUserList, HGPermissionPackage.HGUserDirectoryRead),
		NewRouteSpec("profile", http.MethodGet, UserProfileModuleBasePath, "/account", true, "获取用户账号安全信息", userHandler.GetSecurityInfo),
		NewRouteSpec("profile", http.MethodPut, UserProfileModuleBasePath, "/update", true, "更新用户资料", userHandler.UpdateProfile),
		NewRouteSpec("profile", http.MethodPut, UserProfileModuleBasePath, "/security", true, "更新账号安全信息", userHandler.UpdateSecurity),
		NewRouteSpec("profile", http.MethodGet, UserProfileModuleBasePath, "/avatar", true, "获取本人头像", userHandler.Avatar),
		NewRouteSpec("profile", http.MethodPost, UserProfileModuleBasePath, "/avatar", true, "上传本人头像", userHandler.Avatar),
		NewRouteSpec("profile", http.MethodGet, UserProfileModuleBasePath, "/addresses", true, "获取本人收货地址列表", userHandler.HGListAddresses),
		NewRouteSpec("profile", http.MethodPost, UserProfileModuleBasePath, "/addresses", true, "新增本人收货地址", userHandler.HGCreateAddress),
		NewRouteSpec("profile", http.MethodGet, UserProfileModuleBasePath, "/addresses/detail", true, "获取本人收货地址", userHandler.HGGetAddress),
		NewRouteSpec("profile", http.MethodPut, UserProfileModuleBasePath, "/addresses/detail", true, "修改本人收货地址", userHandler.HGUpdateAddress),
		NewRouteSpec("profile", http.MethodDelete, UserProfileModuleBasePath, "/addresses/detail", true, "删除本人收货地址", userHandler.HGDeleteAddress),
		NewRouteSpec("profile", http.MethodPut, UserProfileModuleBasePath, "/addresses/default", true, "设置本人默认地址", userHandler.HGSetDefaultAddress),
	}
}
