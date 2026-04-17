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
	return NewAuthRouteInterceptorGroup(userHandler)
}

// NewAuthRouteInterceptorGroup 注册认证模块路由并装配拦截器链路。
func NewAuthRouteInterceptorGroup(userHandler *UserHandlerPackage.UserHandler) http.Handler {
	specs := authRoutes(userHandler)
	publicMux := http.NewServeMux()
	bindRouteSpecs(publicMux, specs)

	guarded := HGMiddlewarePackage.APIGuardInterceptor(HGServerPackage.PublicAPIRules())(publicMux)

	// 外层统一打 TID/日志/恢复/JSON 头，确保鉴权失败请求也可追踪。
	return HGMiddlewarePackage.ChainInterceptors(
		guarded,
		HGMiddlewarePackage.RequestTIDInterceptor,
		HGMiddlewarePackage.AccessLogInterceptor,
		HGMiddlewarePackage.RecoverInterceptor,
		HGMiddlewarePackage.JSONHeaderInterceptor,
	)
}

// AuthRouteCatalog 返回 auth 模块完整可调用路径清单。
func AuthRouteCatalog() []HGRouteCatalogItem {
	return buildRouteCatalogItems(authRoutes(nil))
}

// authRoutes 返回 auth 模块完整路由定义。
// 这里会同时保存子路径（用于模块内注册）和完整路径（用于目录展示与联调）。
func authRoutes(userHandler *UserHandlerPackage.UserHandler) []hgRouteSpec {
	if userHandler == nil {
		return []hgRouteSpec{
			newRouteSpec("auth", http.MethodGet, AuthModuleBasePath, "/send_code", false, "发送登录/注册验证码", nil),
			newRouteSpec("auth", http.MethodPost, AuthModuleBasePath, "/register", false, "用户注册", nil),
			newRouteSpec("auth", http.MethodPost, AuthModuleBasePath, "/login", false, "用户登录", nil),
		}
	}

	return []hgRouteSpec{
		newRouteSpec("auth", http.MethodGet, AuthModuleBasePath, "/send_code", false, "发送登录/注册验证码", userHandler.SendCode),
		newRouteSpec("auth", http.MethodPost, AuthModuleBasePath, "/register", false, "用户注册", userHandler.RegisterHandlerV3),
		newRouteSpec("auth", http.MethodPost, AuthModuleBasePath, "/login", false, "用户登录", userHandler.Login),
	}
}
