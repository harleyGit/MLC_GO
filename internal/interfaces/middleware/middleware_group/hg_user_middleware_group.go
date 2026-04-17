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

// UserMiddlewareGroup 注册用户模块路由并装配鉴权链路。
func UserMiddlewareGroup(userHandler *UserHandlerPackage.UserHandler) http.Handler {
	userMux := http.NewServeMux()
	for _, route := range userRouteSpecs(userHandler) {
		userMux.HandleFunc(route.Path, route.Handler)
	}

	guard := HGMiddlewarePackage.NewAPIGuard(HGServerPackage.UserMethodRules())

	authHandler := UserJWTMiddlewarePackage.AuthMiddleware(userMux)
	tidHandler := HGMiddlewarePackage.TIDMiddleware(authHandler)
	jsonHandler := HGMiddlewarePackage.JSONHeaderMiddleware(tidHandler)

	return guard.MethodGuardMiddlewareV3(jsonHandler)
}

// UserRouteCatalog 返回 user/profile 模块完整可调用路径清单。
func UserRouteCatalog(basePrefix string) []HGRouteCatalogItem {
	specs := userRouteSpecs(nil)
	items := make([]HGRouteCatalogItem, 0, len(specs))
	for _, spec := range specs {
		items = append(items, HGRouteCatalogItem{
			Group:    "profile",
			Method:   spec.Method,
			Path:     joinRoutePath(basePrefix, spec.Path),
			NeedAuth: spec.NeedAuth,
			Summary:  spec.Summary,
		})
	}

	return items
}

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
