/*
* @Author: GangHuang harleysor@qq.com
* @Date: 2026-01-26 18:10:07
  - @LastEditors: GangHuang harleysor@qq.com
  - @LastEditTime: 2026-02-01 16:43:26

* @FilePath: /MLC_GO/internal/interfaces/middleware/middleware_group/hg_tid_middle_group.go
* @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE

* Auth 无关接口
*/
package HGMiddlewareGroupPackage

import (
	HGMiddlewarePackage "MLC_GO/internal/interfaces/middleware"
	UserHandlerPackage "MLC_GO/internal/modules/user/handler"
	HGServerPackage "MLC_GO/server"
	"net/http"
)

// AuthMiddlewareGroup 注册认证模块路由并装配公共中间件链路。
func AuthMiddlewareGroup(userHandler *UserHandlerPackage.UserHandler) http.Handler {
	publicMux := http.NewServeMux()
	for _, route := range authRouteSpecs(userHandler) {
		publicMux.HandleFunc(route.Path, route.Handler)
	}

	guard := HGMiddlewarePackage.NewAPIGuard(HGServerPackage.PublicAPIRules())

	recoverHandler := HGMiddlewarePackage.RecoverMiddleware(publicMux)
	loggerHandler := HGMiddlewarePackage.LoggerMiddleware(recoverHandler)
	tidHandler := HGMiddlewarePackage.TIDMiddleware(loggerHandler)
	jsonHandler := HGMiddlewarePackage.JSONHeaderMiddleware(tidHandler)

	return guard.MethodGuardMiddlewareV3(jsonHandler)
}

// AuthRouteCatalog 返回 auth 模块完整可调用路径清单。
func AuthRouteCatalog(basePrefix string) []HGRouteCatalogItem {
	specs := authRouteSpecs(nil)
	items := make([]HGRouteCatalogItem, 0, len(specs))
	for _, spec := range specs {
		items = append(items, HGRouteCatalogItem{
			Group:    "auth",
			Method:   spec.Method,
			Path:     joinRoutePath(basePrefix, spec.Path),
			NeedAuth: spec.NeedAuth,
			Summary:  spec.Summary,
		})
	}

	return items
}

func authRouteSpecs(userHandler *UserHandlerPackage.UserHandler) []hgRouteSpec {
	if userHandler == nil {
		return []hgRouteSpec{
			{Method: http.MethodGet, Path: "/send_code", NeedAuth: false, Summary: "发送登录/注册验证码"},
			{Method: http.MethodPost, Path: "/register", NeedAuth: false, Summary: "用户注册"},
			{Method: http.MethodPost, Path: "/login", NeedAuth: false, Summary: "用户登录"},
		}
	}

	return []hgRouteSpec{
		{Method: http.MethodGet, Path: "/send_code", NeedAuth: false, Summary: "发送登录/注册验证码", Handler: userHandler.SendCode},
		{Method: http.MethodPost, Path: "/register", NeedAuth: false, Summary: "用户注册", Handler: userHandler.RegisterHandlerV3},
		{Method: http.MethodPost, Path: "/login", NeedAuth: false, Summary: "用户登录", Handler: userHandler.Login},
	}
}
