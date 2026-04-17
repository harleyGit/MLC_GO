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
	specs := userRouteSpecs(userHandler)
	userMux := http.NewServeMux()
	bindRouteSpecs(userMux, specs)

	guard := HGMiddlewarePackage.NewAPIGuard(HGServerPackage.UserMethodRules())

	chain := chainMiddlewares(
		userMux,
		UserJWTMiddlewarePackage.AuthMiddleware,
		HGMiddlewarePackage.TIDMiddleware,
		HGMiddlewarePackage.JSONHeaderMiddleware,
	)

	return guard.MethodGuardMiddlewareV3(chain)
}

// UserRouteCatalog 返回 user/profile 模块完整可调用路径清单。
func UserRouteCatalog(basePrefix string) []HGRouteCatalogItem {
	return buildRouteCatalogItems("profile", basePrefix, userRouteSpecs(nil))
}

// userRouteSpecs 返回 profile 模块内部子路由元信息。
// 注意：Path 仅是模块内子路径（如 /list），完整路径由根前缀拼接（如 /api/v1/profile/list）。
func userRouteSpecs(userHandler *UserHandlerPackage.UserHandler) []hgRouteSpec {
	if userHandler == nil {
		return []hgRouteSpec{
			{Method: http.MethodGet, Path: "/info", NeedAuth: true, Summary: "获取当前用户信息"},
			{Method: http.MethodGet, Path: "/list", NeedAuth: true, Summary: "获取用户分页列表"},
		}
	}

	return []hgRouteSpec{
		{Method: http.MethodGet, Path: "/info", NeedAuth: true, Summary: "获取当前用户信息", Handler: userHandler.Profile},
		{Method: http.MethodGet, Path: "/list", NeedAuth: true, Summary: "获取用户分页列表", Handler: userHandler.GetUserList},
	}
}
