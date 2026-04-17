/*
* @Author: GangHuang harleysor@qq.com
* @Date: 2026-01-26 19:48:25
  - @LastEditors: GangHuang harleysor@qq.com
  - @LastEditTime: 2026-03-01 18:56:38

* @FilePath: /MLC_GO/internal/interfaces/middleware/middleware_group/hg_user_middle_group.go
* @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE

* 需要用户登录的接口
*/
package HGMiddlewareGroupPackage

import (
	HGMiddlewarePackage "MLC_GO/internal/interfaces/middleware"
	UserHandlerPackage "MLC_GO/internal/modules/user/handler"
	UserJWTMiddlewarePackage "MLC_GO/internal/modules/user/middleware"
	HGServerPackage "MLC_GO/server"
	"net/http"
)

const (
	// UserProfileModuleBasePath 是用户资料模块对外暴露的统一 API 前缀。
	UserProfileModuleBasePath = "/api/v1/profile"
)

// UserMiddlewareGroup 注册用户模块路由并装配鉴权链路。
func UserMiddlewareGroup(userHandler *UserHandlerPackage.UserHandler) http.Handler {
	return NewUserRouteInterceptorGroup(userHandler)
}

// NewUserRouteInterceptorGroup 注册用户模块路由并装配鉴权拦截器链路。
func NewUserRouteInterceptorGroup(userHandler *UserHandlerPackage.UserHandler) http.Handler {
	specs := userRoutes(userHandler)
	userMux := http.NewServeMux()
	bindRouteSpecs(userMux, specs)

	protected := HGMiddlewarePackage.ChainInterceptors(
		userMux,
		UserJWTMiddlewarePackage.AuthMiddleware,
	)
	guarded := HGMiddlewarePackage.APIGuardInterceptor(HGServerPackage.UserMethodRules())(protected)

	// 外层统一打 TID/日志/恢复/JSON 头，确保鉴权失败请求也可追踪。
	return HGMiddlewarePackage.ChainInterceptors(
		guarded,
		HGMiddlewarePackage.RequestTIDInterceptor,
		HGMiddlewarePackage.AccessLogInterceptor,
		HGMiddlewarePackage.RecoverInterceptor,
		HGMiddlewarePackage.JSONHeaderInterceptor,
	)
}

// UserRouteCatalog 返回 user/profile 模块完整可调用路径清单。
func UserRouteCatalog() []HGRouteCatalogItem {
	return buildRouteCatalogItems(userRoutes(nil))
}

// userRoutes 返回 profile 模块完整路由定义。
// 这里会同时保存子路径（用于模块内注册）和完整路径（用于目录展示与联调）。
func userRoutes(userHandler *UserHandlerPackage.UserHandler) []hgRouteSpec {
	if userHandler == nil {
		return []hgRouteSpec{
			newRouteSpec("profile", http.MethodGet, UserProfileModuleBasePath, "/info", true, "获取当前用户信息", nil),
			newRouteSpec("profile", http.MethodGet, UserProfileModuleBasePath, "/list", true, "获取用户分页列表", nil),
		}
	}

	return []hgRouteSpec{
		newRouteSpec("profile", http.MethodGet, UserProfileModuleBasePath, "/info", true, "获取当前用户信息", userHandler.Profile),
		newRouteSpec("profile", http.MethodGet, UserProfileModuleBasePath, "/list", true, "获取用户分页列表", userHandler.GetUserList),
	}
}
