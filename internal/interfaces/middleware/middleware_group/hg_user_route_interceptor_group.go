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

// NewUserRouteGroup 注册用户模块路由并装配鉴权链路。
func NewUserRouteGroup(userHandler *UserHandlerPackage.UserHandler) http.Handler {
	specs := userRoutes(userHandler)
	userMux := http.NewServeMux()
	BindRouteSpecs(userMux, specs)

	protected := HGMiddlewarePackage.Chain(
		userMux,
		UserJWTMiddlewarePackage.AuthMiddleware,
	)
	guarded := HGMiddlewarePackage.APIGuardMiddleware(HGServerPackage.UserMethodRules())(protected)

	// 外层统一打 TID/日志/恢复/JSON 头，确保鉴权失败请求也可追踪。
	return HGMiddlewarePackage.Chain(
		guarded,
		HGMiddlewarePackage.RequestIDMiddleware,
		HGMiddlewarePackage.AccessLogMiddleware,
		HGMiddlewarePackage.RecoverMiddleware,
		HGMiddlewarePackage.JSONHeaderMiddleware,
	)
}

// NewUserRouteInterceptorGroup 兼容旧方法名。
func NewUserRouteInterceptorGroup(userHandler *UserHandlerPackage.UserHandler) http.Handler {
	return NewUserRouteGroup(userHandler)
}

// UserMiddlewareGroup 兼容旧方法名。
func UserMiddlewareGroup(userHandler *UserHandlerPackage.UserHandler) http.Handler {
	return NewUserRouteGroup(userHandler)
}

// UserRouteCatalog 返回 user/profile 模块完整可调用路径清单。
func UserRouteCatalog() []HGRouteCatalogItem {
	return BuildRouteCatalogItems(userRoutes(nil))
}

// userRoutes 返回 profile 模块完整路由定义。
func userRoutes(userHandler *UserHandlerPackage.UserHandler) []RouteSpec {
	if userHandler == nil {
		return []RouteSpec{
			NewRouteSpec("profile", http.MethodGet, UserProfileModuleBasePath, "/info", true, "获取当前用户信息", nil),
			NewRouteSpec("profile", http.MethodGet, UserProfileModuleBasePath, "/list", true, "获取用户分页列表", nil),
			NewRouteSpec("profile", http.MethodPut, UserProfileModuleBasePath, "/update", true, "更新用户资料", nil),
		}
	}

	return []RouteSpec{
		NewRouteSpec("profile", http.MethodGet, UserProfileModuleBasePath, "/info", true, "获取当前用户信息", userHandler.Profile),
		NewRouteSpec("profile", http.MethodGet, UserProfileModuleBasePath, "/list", true, "获取用户分页列表", userHandler.GetUserList),
		NewRouteSpec("profile", http.MethodPut, UserProfileModuleBasePath, "/update", true, "更新用户资料", userHandler.UpdateProfile),
	}
}
