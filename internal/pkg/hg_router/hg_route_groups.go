package HGRouterPackage

import (
	BilibiliHandlerPackage "MLC_GO/internal/modules/bilibili/handler"
	OpsHandlerPackage "MLC_GO/internal/modules/ops/handler"
	UserHandlerPackage "MLC_GO/internal/modules/user/handler"
	UserJWTMiddlewarePackage "MLC_GO/internal/modules/user/middleware"
	VideoCommentHandlerPackage "MLC_GO/internal/modules/video_comment/handler"
	VideoDanmakuHandlerPackage "MLC_GO/internal/modules/video_danmaku/handler"
	VideoInteractionHandlerPackage "MLC_GO/internal/modules/video_interaction/handler"
	VideoRecommendHandlerPackage "MLC_GO/internal/modules/video_recommend/handler"
	VideoUploadHandlerPackage "MLC_GO/internal/modules/video_upload/handler"
	HGMiddlewarePackage "MLC_GO/internal/pkg/middleware"
	HGServerPackage "MLC_GO/internal/pkg/server"
	"net/http"
)

// region 模块路径常量
const (
	AuthModuleBasePath             = "/api/v1/auth"         // 认证模块基础路径
	UserProfileModuleBasePath      = "/api/v1/profile"      // 用户信息模块基础路径
	VideoUploadModuleBasePath      = "/api/v1/video_upload" // 视频投稿模块基础路径
	OpsModuleBasePath              = "/api/v1/ops"          // 运维管理模块基础路径
	VideoInteractionModuleBasePath = "/api/v1/video_interactions"
	VideoCommentModuleBasePath     = "/api/v1/video_comments"
	VideoDanmakuModuleBasePath     = "/api/v1/video_danmaku"
	BilibiliModuleBasePath         = "/api/v1/bilibili"
	VideoRecommendModuleBasePath   = "/api/v1/video_recommend"
)

// endregion

// region 路由组构建器（高性能，支持百万级并发），RouteGroupConfig 路由组配置。
type RouteGroupConfig struct {
	BasePath       string
	Rules          []HGMiddlewarePackage.APIRule
	AuthMiddleware HGMiddlewarePackage.Middleware // 可选，nil 表示不需要认证
}

// baseMiddlewares 只保留模块响应协议；请求 ID、日志和 panic 边界由根入口统一执行一次。
var baseMiddlewares = []HGMiddlewarePackage.Middleware{
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

// NewBilibiliRouteGroup 注册 Bilibili 作者空间公开读路由。
func NewBilibiliRouteGroup(handler *BilibiliHandlerPackage.Handler) http.Handler {
	return NewRouteGroup(RouteGroupConfig{BasePath: BilibiliModuleBasePath, Rules: HGServerPackage.BilibiliMethodRules()}, bilibiliRoutes(handler))
}

// BilibiliRouteCatalog 返回 Bilibili 作者空间完整路由清单。
func BilibiliRouteCatalog() []RouteCatalogItem { return BuildRouteCatalogItems(bilibiliRoutes(nil)) }

// NewVideoRecommendRouteGroup 注册认证视频推荐流路由。
func NewVideoRecommendRouteGroup(handler *VideoRecommendHandlerPackage.Handler) http.Handler {
	return NewRouteGroup(RouteGroupConfig{BasePath: VideoRecommendModuleBasePath, Rules: HGServerPackage.VideoRecommendMethodRules(), AuthMiddleware: UserJWTMiddlewarePackage.AuthMiddleware}, videoRecommendRoutes(handler))
}

// VideoRecommendRouteCatalog 返回视频推荐模块完整路由清单。
func VideoRecommendRouteCatalog() []RouteCatalogItem {
	return BuildRouteCatalogItems(videoRecommendRoutes(nil))
}

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

func NewVideoInteractionRouteGroup(handler *VideoInteractionHandlerPackage.Handler) http.Handler {
	return NewRouteGroup(RouteGroupConfig{
		BasePath: VideoInteractionModuleBasePath, Rules: HGServerPackage.VideoInteractionMethodRules(), AuthMiddleware: UserJWTMiddlewarePackage.AuthMiddleware,
	}, videoInteractionRoutes(handler))
}

func VideoInteractionRouteCatalog() []RouteCatalogItem {
	return BuildRouteCatalogItems(videoInteractionRoutes(nil))
}

// NewVideoCommentRouteGroup 为评论路由统一应用方法白名单和 JWT 认证。
func NewVideoCommentRouteGroup(handler *VideoCommentHandlerPackage.Handler) http.Handler {
	return NewRouteGroup(RouteGroupConfig{
		BasePath: VideoCommentModuleBasePath, Rules: HGServerPackage.VideoCommentMethodRules(), AuthMiddleware: UserJWTMiddlewarePackage.AuthMiddleware,
	}, videoCommentRoutes(handler))
}

// VideoCommentRouteCatalog 返回视频评论模块完整可调用路径清单。
func VideoCommentRouteCatalog() []RouteCatalogItem {
	return BuildRouteCatalogItems(videoCommentRoutes(nil))
}

// NewVideoDanmakuRouteGroup 为弹幕历史、创建和票据接口统一应用 API Guard 与 JWT。
func NewVideoDanmakuRouteGroup(handler *VideoDanmakuHandlerPackage.Handler) http.Handler {
	return NewRouteGroup(RouteGroupConfig{BasePath: VideoDanmakuModuleBasePath, Rules: HGServerPackage.VideoDanmakuMethodRules(), AuthMiddleware: UserJWTMiddlewarePackage.AuthMiddleware}, videoDanmakuRoutes(handler))
}

// VideoDanmakuRouteCatalog 返回 HTTP 与 WebSocket 的完整公开路径清单。
func VideoDanmakuRouteCatalog() []RouteCatalogItem {
	return BuildRouteCatalogItems(videoDanmakuRoutes(nil))
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
