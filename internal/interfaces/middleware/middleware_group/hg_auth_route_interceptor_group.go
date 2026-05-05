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

// NewAuthRouteGroup 注册认证模块路由并装配中间件链路。
func NewAuthRouteGroup(userHandler *UserHandlerPackage.UserHandler) http.Handler {
	specs := authRoutes(userHandler)
	publicMux := http.NewServeMux()
	BindRouteSpecs(publicMux, specs)

	guarded := HGMiddlewarePackage.APIGuardMiddleware(HGServerPackage.PublicAPIRules())(publicMux)

	// 外层统一打 TID/日志/恢复/JSON 头，确保鉴权失败请求也可追踪。
	return HGMiddlewarePackage.Chain(
		guarded,
		HGMiddlewarePackage.RequestIDMiddleware,
		HGMiddlewarePackage.AccessLogMiddleware,
		HGMiddlewarePackage.RecoverMiddleware,
		HGMiddlewarePackage.JSONHeaderMiddleware,
	)
}

// NewAuthRouteInterceptorGroup 兼容旧方法名。
func NewAuthRouteInterceptorGroup(userHandler *UserHandlerPackage.UserHandler) http.Handler {
	return NewAuthRouteGroup(userHandler)
}

// AuthMiddlewareGroup 兼容旧方法名。
func AuthMiddlewareGroup(userHandler *UserHandlerPackage.UserHandler) http.Handler {
	return NewAuthRouteGroup(userHandler)
}

// AuthRouteCatalog 返回 auth 模块完整可调用路径清单。
func AuthRouteCatalog() []HGRouteCatalogItem {
	return BuildRouteCatalogItems(authRoutes(nil))
}

// authRoutes 返回 auth 模块完整路由定义。
func authRoutes(userHandler *UserHandlerPackage.UserHandler) []RouteSpec {
	if userHandler == nil {
		return []RouteSpec{
			NewRouteSpec("auth", http.MethodGet, AuthModuleBasePath, "/send_code", false, "发送登录/注册验证码", nil),
			NewRouteSpec("auth", http.MethodPost, AuthModuleBasePath, "/register", false, "用户注册", nil),
			NewRouteSpec("auth", http.MethodPost, AuthModuleBasePath, "/login", false, "用户登录", nil),
		}
	}

	return []RouteSpec{
		NewRouteSpec("auth", http.MethodGet, AuthModuleBasePath, "/send_code", false, "发送登录/注册验证码", userHandler.SendCode),
		NewRouteSpec("auth", http.MethodPost, AuthModuleBasePath, "/register", false, "用户注册", userHandler.RegisterHandlerV3),
		NewRouteSpec("auth", http.MethodPost, AuthModuleBasePath, "/login", false, "用户登录", userHandler.Login),
	}
}
