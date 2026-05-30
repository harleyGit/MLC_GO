package HGMiddlewareGroupPackage

import (
	HGMiddlewarePackage "MLC_GO/internal/interfaces/middleware"
	OpsHandlerPackage "MLC_GO/internal/modules/ops/handler"
	UserHandlerPackage "MLC_GO/internal/modules/user/handler"
	UserJWTMiddlewarePackage "MLC_GO/internal/modules/user/middleware"
	VideoUploadHandlerPackage "MLC_GO/internal/modules/video_upload/handler"
	HGServerPackage "MLC_GO/server"
	"net/http"
)

// region 模块路径常量
const (
	AuthModuleBasePath        = "/api/v1/auth"         // 认证模块基础路径
	UserProfileModuleBasePath = "/api/v1/profile"      // 用户信息模块基础路径
	VideoUploadModuleBasePath = "/api/v1/video_upload" // 视频投稿模块基础路径
	OpsModuleBasePath         = "/api/v1/ops"          // 运维管理模块基础路径
)

// endregion

// region 路由组构建器（高性能，支持百万级并发），RouteGroupConfig 路由组配置。
type RouteGroupConfig struct {
	BasePath       string
	Rules          []HGMiddlewarePackage.APIRule
	AuthMiddleware HGMiddlewarePackage.Middleware // 可选，nil 表示不需要认证
}

// baseMiddlewares 预编译的基础中间件链，启动时构建一次，所有请求复用。
var baseMiddlewares = []HGMiddlewarePackage.Middleware{
	HGMiddlewarePackage.RequestIDMiddleware,
	HGMiddlewarePackage.AccessLogMiddleware,
	HGMiddlewarePackage.RecoverMiddleware,
	HGMiddlewarePackage.JSONHeaderMiddleware,
}

// NewRouteGroup 通用路由组构建器，消除重复代码。
// 性能优化：中间件链在启动时构建一次，请求期零分配。
func NewRouteGroup(config RouteGroupConfig, specs []RouteSpec) http.Handler {
	mux := http.NewServeMux()
	BindRouteSpecs(mux, specs)

	// 1. 业务中间件（可选）
	var handler http.Handler = mux
	if config.AuthMiddleware != nil {
		handler = HGMiddlewarePackage.Chain(mux, config.AuthMiddleware)
	}

	// 2. API Guard
	if config.Rules != nil {
		handler = HGMiddlewarePackage.APIGuardMiddleware(config.Rules)(handler)
	}

	// 3. 基础中间件链（预编译，零分配）
	return HGMiddlewarePackage.Chain(handler, baseMiddlewares...)
}

// endregion

// region Auth 路由组

// NewAuthRouteGroup 注册认证模块路由（公开接口，无需认证）。
func NewAuthRouteGroup(userHandler *UserHandlerPackage.HGUserHandler) http.Handler {
	return NewRouteGroup(
		RouteGroupConfig{
			BasePath: AuthModuleBasePath,
			Rules:    HGServerPackage.PublicAPIRules(),
		},
		authRoutes(userHandler),
	)
}

// NewAuthRouteInterceptorGroup 兼容旧方法名。
func NewAuthRouteInterceptorGroup(userHandler *UserHandlerPackage.HGUserHandler) http.Handler {
	return NewAuthRouteGroup(userHandler)
}

// AuthMiddlewareGroup 兼容旧方法名。
func AuthMiddlewareGroup(userHandler *UserHandlerPackage.HGUserHandler) http.Handler {
	return NewAuthRouteGroup(userHandler)
}

// AuthRouteCatalog 返回 auth 模块完整可调用路径清单。
func AuthRouteCatalog() []RouteCatalogItem {
	return BuildRouteCatalogItems(authRoutes(nil))
}

// authRoutes 返回 auth 模块完整路由定义。
func authRoutes(userHandler *UserHandlerPackage.HGUserHandler) []RouteSpec {
	if userHandler == nil {
		return []RouteSpec{
			NewRouteSpec("auth", http.MethodGet, AuthModuleBasePath, "/send_code", false, "发送登录/注册验证码", nil),
			NewRouteSpec("auth", http.MethodPost, AuthModuleBasePath, "/register", false, "用户注册", nil),
			NewRouteSpec("auth", http.MethodPost, AuthModuleBasePath, "/login", false, "用户登录", nil),
			NewRouteSpec("auth", http.MethodPost, AuthModuleBasePath, "/refresh", false, "刷新 Token", nil),
			NewRouteSpec("auth", http.MethodGet, AuthModuleBasePath, "/send_reset_code", false, "发送忘记密码验证码", nil),
			NewRouteSpec("auth", http.MethodPost, AuthModuleBasePath, "/reset_password", false, "忘记密码重置", nil),
		}
	}

	return []RouteSpec{
		NewRouteSpec("auth", http.MethodGet, AuthModuleBasePath, "/send_code", false, "发送登录/注册验证码", userHandler.SendCode),
		NewRouteSpec("auth", http.MethodPost, AuthModuleBasePath, "/register", false, "用户注册", userHandler.RegisterHandlerV3),
		NewRouteSpec("auth", http.MethodPost, AuthModuleBasePath, "/login", false, "用户登录", userHandler.Login),
		NewRouteSpec("auth", http.MethodPost, AuthModuleBasePath, "/refresh", false, "刷新 Token", userHandler.RefreshToken),
		NewRouteSpec("auth", http.MethodGet, AuthModuleBasePath, "/send_reset_code", false, "发送忘记密码验证码", userHandler.SendResetPasswordCode),
		NewRouteSpec("auth", http.MethodPost, AuthModuleBasePath, "/reset_password", false, "忘记密码重置", userHandler.ResetPassword),
	}
}

// endregion

// region User 路由组

// NewUserRouteGroup 注册用户模块路由（需要认证）。
func NewUserRouteGroup(userHandler *UserHandlerPackage.HGUserHandler) http.Handler {
	return NewRouteGroup(
		RouteGroupConfig{
			BasePath:       UserProfileModuleBasePath,
			Rules:          HGServerPackage.UserMethodRules(),
			AuthMiddleware: UserJWTMiddlewarePackage.AuthMiddleware,
		},
		userRoutes(userHandler),
	)
}

// NewUserRouteInterceptorGroup 兼容旧方法名。
func NewUserRouteInterceptorGroup(userHandler *UserHandlerPackage.HGUserHandler) http.Handler {
	return NewUserRouteGroup(userHandler)
}

// UserMiddlewareGroup 兼容旧方法名。
func UserMiddlewareGroup(userHandler *UserHandlerPackage.HGUserHandler) http.Handler {
	return NewUserRouteGroup(userHandler)
}

// UserRouteCatalog 返回 user/profile 模块完整可调用路径清单。
func UserRouteCatalog() []RouteCatalogItem {
	return BuildRouteCatalogItems(userRoutes(nil))
}

// userRoutes 返回 profile 模块完整路由定义。
func userRoutes(userHandler *UserHandlerPackage.HGUserHandler) []RouteSpec {
	if userHandler == nil {
		return []RouteSpec{
			NewRouteSpec("profile", http.MethodGet, UserProfileModuleBasePath, "/info", true, "获取当前用户信息", nil),
			NewRouteSpec("profile", http.MethodGet, UserProfileModuleBasePath, "/list", true, "获取用户分页列表", nil),
			NewRouteSpec("profile", http.MethodGet, UserProfileModuleBasePath, "/account", true, "获取用户账号安全信息", nil),
			NewRouteSpec("profile", http.MethodPut, UserProfileModuleBasePath, "/update", true, "更新用户资料", nil),
			NewRouteSpec("profile", http.MethodPut, UserProfileModuleBasePath, "/security", true, "更新账号安全信息", nil),
			NewRouteSpec("profile", http.MethodGet, UserProfileModuleBasePath, "/avatar", true, "头像操作（POST上传/GET获取）", nil),
		}
	}

	return []RouteSpec{
		NewRouteSpec("profile", http.MethodGet, UserProfileModuleBasePath, "/info", true, "获取当前用户信息", userHandler.Profile),
		NewRouteSpec("profile", http.MethodGet, UserProfileModuleBasePath, "/list", true, "获取用户分页列表", userHandler.GetUserList),
		NewRouteSpec("profile", http.MethodGet, UserProfileModuleBasePath, "/account", true, "获取用户账号安全信息", userHandler.GetSecurityInfo),
		NewRouteSpec("profile", http.MethodPut, UserProfileModuleBasePath, "/update", true, "更新用户资料", userHandler.UpdateProfile),
		NewRouteSpec("profile", http.MethodPut, UserProfileModuleBasePath, "/security", true, "更新账号安全信息", userHandler.UpdateSecurity),
		NewRouteSpec("profile", http.MethodGet, UserProfileModuleBasePath, "/avatar", true, "头像操作（POST上传/GET获取）", userHandler.Avatar),
	}
}

// endregion

// region VideoUpload 路由组

func NewVideoUploadRouteGroup(videoUploadHandler *VideoUploadHandlerPackage.Handler) http.Handler {
	return NewRouteGroup(
		RouteGroupConfig{
			BasePath:       VideoUploadModuleBasePath,
			Rules:          HGServerPackage.VideoUploadMethodRules(),
			AuthMiddleware: UserJWTMiddlewarePackage.AuthMiddleware,
		},
		videoUploadRoutes(videoUploadHandler),
	)
}

func VideoUploadRouteCatalog() []RouteCatalogItem {
	return BuildRouteCatalogItems(videoUploadRoutes(nil))
}

func videoUploadRoutes(videoUploadHandler *VideoUploadHandlerPackage.Handler) []RouteSpec {
	if videoUploadHandler == nil {
		return []RouteSpec{
			NewRouteSpec("video_upload", http.MethodPost, VideoUploadModuleBasePath, "/upload", true, "上传视频文件", nil),
			NewRouteSpec("video_upload", http.MethodPost, VideoUploadModuleBasePath, "/draft", true, "保存视频稿件草稿", nil),
			NewRouteSpec("video_upload", http.MethodPost, VideoUploadModuleBasePath, "/submit", true, "提交视频稿件审核", nil),
			NewRouteSpec("video_upload", http.MethodGet, VideoUploadModuleBasePath, "/list", true, "获取视频列表", nil),
			NewRouteSpec("video_upload", http.MethodPost, VideoUploadModuleBasePath, "/cover", true, "上传封面图", nil),
		}
	}

	return []RouteSpec{
		NewRouteSpec("video_upload", http.MethodPost, VideoUploadModuleBasePath, "/upload", true, "上传视频文件", videoUploadHandler.UploadVideo),
		NewRouteSpec("video_upload", http.MethodPost, VideoUploadModuleBasePath, "/draft", true, "保存视频稿件草稿", videoUploadHandler.SaveDraft),
		NewRouteSpec("video_upload", http.MethodPost, VideoUploadModuleBasePath, "/submit", true, "提交视频稿件审核", videoUploadHandler.Submit),
		NewRouteSpec("video_upload", http.MethodGet, VideoUploadModuleBasePath, "/list", true, "获取视频列表", videoUploadHandler.GetVideoList),
		NewRouteSpec("video_upload", http.MethodPost, VideoUploadModuleBasePath, "/cover", true, "上传封面图", videoUploadHandler.UploadCover),
	}
}

// endregion

// region Ops 路由组

// NewOpsRouteGroup 注册运维管理模块路由（需要认证）。
func NewOpsRouteGroup(opsHandler *OpsHandlerPackage.Handler) http.Handler {
	return NewRouteGroup(
		RouteGroupConfig{
			BasePath:       OpsModuleBasePath,
			Rules:          HGServerPackage.OpsMethodRules(),
			AuthMiddleware: UserJWTMiddlewarePackage.AuthMiddleware,
		},
		opsRoutes(opsHandler),
	)
}

// OpsRouteCatalog 返回 ops 模块完整可调用路径清单。
func OpsRouteCatalog() []RouteCatalogItem {
	return BuildRouteCatalogItems(opsRoutes(nil))
}

// opsRoutes 返回 ops 模块完整路由定义。
func opsRoutes(opsHandler *OpsHandlerPackage.Handler) []RouteSpec {
	if opsHandler == nil {
		return []RouteSpec{
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/roles", true, "创建角色", nil),
			NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/roles", true, "获取角色列表", nil),
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/users/roles", true, "分配用户角色", nil),
			NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/users/roles", true, "获取用户角色", nil),
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/menus", true, "创建菜单", nil),
			NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/menus", true, "获取菜单列表", nil),
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/roles/permissions", true, "分配角色权限", nil),
			NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/roles/permissions", true, "获取角色权限", nil),
			NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/files/upload", true, "上传文件", nil),
			NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/files", true, "获取文件列表", nil),
		}
	}

	return []RouteSpec{
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/roles", true, "创建角色", opsHandler.CreateRole),
		NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/roles", true, "获取角色列表", opsHandler.GetRoleList),
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/users/roles", true, "分配用户角色", opsHandler.AssignUserRoles),
		NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/users/roles", true, "获取用户角色", opsHandler.GetUserRoles),
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/menus", true, "创建菜单", opsHandler.CreateMenu),
		NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/menus", true, "获取菜单列表", opsHandler.GetMenuList),
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/roles/permissions", true, "分配角色权限", opsHandler.AssignRolePermissions),
		NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/roles/permissions", true, "获取角色权限", opsHandler.GetRolePermissions),
		NewRouteSpec("ops", http.MethodPost, OpsModuleBasePath, "/files/upload", true, "上传文件", opsHandler.UploadFile),
		NewRouteSpec("ops", http.MethodGet, OpsModuleBasePath, "/files", true, "获取文件列表", opsHandler.GetFileList),
	}
}

// endregion

// region Order 路由组（示例）

/*
const OrderModuleBasePath = "/api/v1/order"

func NewOrderRouteGroup(orderHandler *OrderHandler) http.Handler {
	return NewRouteGroup(
		RouteGroupConfig{
			BasePath:       OrderModuleBasePath,
			Rules:          HGServerPackage.OrderMethodRules(),
			AuthMiddleware: UserJWTMiddlewarePackage.AuthMiddleware,
		},
		[]RouteSpec{
			NewRouteSpec("order", http.MethodPost, OrderModuleBasePath, "/create", true, "创建订单", orderHandler.Create),
			NewRouteSpec("order", http.MethodGet, OrderModuleBasePath, "/list", true, "订单列表", orderHandler.List),
		},
	)
}
*/

// endregion
