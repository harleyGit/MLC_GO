package HGMiddlewareGroupPackage

import (
	HGMiddlewarePackage "MLC_GO/internal/interfaces/middleware"
	UserHandlerPackage "MLC_GO/internal/modules/user/handler"
	UserJWTMiddlewarePackage "MLC_GO/internal/modules/user/middleware"
	HGServerPackage "MLC_GO/server"
	"net/http"
)

// region 模块路径常量

const (
	// AuthModuleBasePath 是认证模块对外暴露的统一 API 前缀。
	AuthModuleBasePath = "/api/v1/auth"

	// UserProfileModuleBasePath 是用户资料模块对外暴露的统一 API 前缀。
	UserProfileModuleBasePath = "/api/v1/profile"
)

// endregion

// region Auth 路由组

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
func AuthRouteCatalog() []RouteCatalogItem {
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

// endregion

// region User 路由组

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
func UserRouteCatalog() []RouteCatalogItem {
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

// endregion

// region Order 路由组（示例，待实现）

/*
const OrderModuleBasePath = "/api/v1/order"

func NewOrderRouteGroup(orderHandler *OrderHandler) http.Handler {
	specs := []RouteSpec{
		NewRouteSpec("order", http.MethodPost, OrderModuleBasePath, "/create", true, "创建订单", orderHandler.Create),
		NewRouteSpec("order", http.MethodGet, OrderModuleBasePath, "/list", true, "订单列表", orderHandler.List),
	}

	orderMux := http.NewServeMux()
	BindRouteSpecs(orderMux, specs)

	protected := HGMiddlewarePackage.Chain(
		orderMux,
		UserJWTMiddlewarePackage.AuthMiddleware,
	)

	return HGMiddlewarePackage.Chain(
		protected,
		HGMiddlewarePackage.RequestIDMiddleware,
		HGMiddlewarePackage.AccessLogMiddleware,
		HGMiddlewarePackage.RecoverMiddleware,
		HGMiddlewarePackage.JSONHeaderMiddleware,
	)
}
*/

// endregion
