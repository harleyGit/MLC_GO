package HGRouterPackage

import (
	UserHandlerPackage "MLC_GO/internal/modules/user/handler"
	HGContextPackage "MLC_GO/internal/pkg/hg_context"
	HGPermissionPackage "MLC_GO/internal/pkg/permission"
	HGResponsePackage "MLC_GO/internal/response"
	"context"
	"net/http"
	"strings"
	"time"
)

// userRoutes 返回 profile 模块完整路由定义。
func userRoutes(userHandler *UserHandlerPackage.HGUserHandler, hgAuthorizers ...HGRouteAuthorizer) []RouteSpec {
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
		NewRouteSpec("profile", http.MethodGet, UserProfileModuleBasePath, "/account", true, "获取用户账号安全信息", hgOwnProfileOnly(userHandler.GetSecurityInfo)),
		NewRouteSpec("profile", http.MethodPut, UserProfileModuleBasePath, "/update", true, "更新用户资料", hgProfileTargetWriteGuard(userHandler.UpdateProfile, hgFirstAuthorizer(hgAuthorizers))),
		NewRouteSpec("profile", http.MethodPut, UserProfileModuleBasePath, "/security", true, "更新账号安全信息", hgProfileTargetWriteGuard(userHandler.UpdateSecurity, hgFirstAuthorizer(hgAuthorizers))),
		NewRouteSpec("profile", http.MethodGet, UserProfileModuleBasePath, "/avatar", true, "获取本人头像", hgOwnProfileOnly(userHandler.Avatar)),
		NewRouteSpec("profile", http.MethodPost, UserProfileModuleBasePath, "/avatar", true, "上传本人头像", hgProfileTargetWriteGuard(userHandler.Avatar, hgFirstAuthorizer(hgAuthorizers))),
		NewRouteSpec("profile", http.MethodGet, UserProfileModuleBasePath, "/addresses", true, "获取本人收货地址列表", userHandler.HGListAddresses),
		NewRouteSpec("profile", http.MethodPost, UserProfileModuleBasePath, "/addresses", true, "新增本人收货地址", userHandler.HGCreateAddress),
		NewRouteSpec("profile", http.MethodGet, UserProfileModuleBasePath, "/addresses/detail", true, "获取本人收货地址", userHandler.HGGetAddress),
		NewRouteSpec("profile", http.MethodPut, UserProfileModuleBasePath, "/addresses/detail", true, "修改本人收货地址", userHandler.HGUpdateAddress),
		NewRouteSpec("profile", http.MethodDelete, UserProfileModuleBasePath, "/addresses/detail", true, "删除本人收货地址", userHandler.HGDeleteAddress),
		NewRouteSpec("profile", http.MethodPut, UserProfileModuleBasePath, "/addresses/default", true, "设置本人默认地址", userHandler.HGSetDefaultAddress),
	}
}

// hgOwnProfileOnly 防止账号安全信息及头像查询借 query user_id 读取他人资料。
func hgOwnProfileOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := HGContextPackage.CurrentUserID(r)
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			HGResponsePackage.FailTokenInvalid(w, r, "登录状态无效")
			return
		}
		if target := strings.TrimSpace(r.URL.Query().Get("user_id")); target != "" && target != actor {
			hgPermissionError(w, r, http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

// hgProfileTargetWriteGuard 本人修改保持原行为；指定其他用户时必须经 IAM 单独授权。
func hgProfileTargetWriteGuard(next http.HandlerFunc, authorizer HGRouteAuthorizer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := HGContextPackage.CurrentUserID(r)
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			HGResponsePackage.FailTokenInvalid(w, r, "登录状态无效")
			return
		}
		if target := strings.TrimSpace(r.URL.Query().Get("user_id")); target != "" && target != actor {
			if authorizer == nil {
				hgPermissionError(w, r, http.StatusServiceUnavailable)
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			allowed, err := authorizer.HasAssetPermission(ctx, actor, string(HGPermissionPackage.HGUserDirectoryWrite))
			if err != nil || ctx.Err() != nil {
				hgPermissionError(w, r, http.StatusServiceUnavailable)
				return
			}
			if !allowed {
				hgPermissionError(w, r, http.StatusForbidden)
				return
			}
		}
		next(w, r)
	}
}
