/*
* @Author: GangHuang harleysor@qq.com
* @Date: 2026-01-26 18:10:07
  - @LastEditors: GangHuang harleysor@qq.com
  - @LastEditTime: 2026-04-18 01:18:00

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

const (
	// AuthModuleBasePath 是认证模块对外暴露的统一 API 前缀。
	AuthModuleBasePath = "/api/v1/auth"
)

// AuthMiddlewareGroup 注册认证模块路由并装配公共中间件链路。
func AuthMiddlewareGroup(userHandler *UserHandlerPackage.UserHandler) http.Handler {
	specs := authRouteSpecs(userHandler)
	publicMux := http.NewServeMux()
	bindRouteSpecs(publicMux, specs)

	guard := HGMiddlewarePackage.NewAPIGuard(HGServerPackage.PublicAPIRules())

	// 链路顺序：Recover -> Logger -> TID -> JSONHeader -> Handler。
	chain := chainMiddlewares(
		publicMux,
		HGMiddlewarePackage.RecoverMiddleware,
		HGMiddlewarePackage.LoggerMiddleware,
		HGMiddlewarePackage.TIDMiddleware,
		HGMiddlewarePackage.JSONHeaderMiddleware,
	)

	return guard.MethodGuardMiddlewareV3(chain)
}

// AuthRouteCatalog 返回 auth 模块完整可调用路径清单。
func AuthRouteCatalog(basePrefix string) []HGRouteCatalogItem {
	return buildRouteCatalogItems("auth", basePrefix, authRouteSpecs(nil))
}

// authRouteSpecs 返回 auth 模块内部子路由元信息。
// 注意：Path 是模块内子路径（如 /login），完整路径需拼接模块前缀（如 /api/v1/auth/login）。
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
