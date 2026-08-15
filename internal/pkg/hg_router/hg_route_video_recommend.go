package HGRouterPackage

import (
	VideoRecommendHandlerPackage "MLC_GO/internal/modules/video_recommend/handler"
	"net/http"
)

// videoRecommendRoutes 返回认证视频推荐模块的固定路由定义。
func videoRecommendRoutes(handler *VideoRecommendHandlerPackage.Handler) []RouteSpec {
	var feed http.HandlerFunc
	if handler != nil {
		feed = handler.Feed
	}
	return []RouteSpec{
		NewRouteSpec("video_recommend", http.MethodGet, VideoRecommendModuleBasePath, "/feed", true, "获取个性化视频推荐流", feed),
	}
}
