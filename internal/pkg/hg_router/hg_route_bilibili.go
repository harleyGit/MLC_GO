package HGRouterPackage

import (
	BilibiliHandlerPackage "MLC_GO/internal/modules/bilibili/handler"
	"net/http"
)

func bilibiliRoutes(handler *BilibiliHandlerPackage.Handler) []RouteSpec {
	if handler == nil {
		return []RouteSpec{
			NewRouteSpec("bilibili", http.MethodGet, BilibiliModuleBasePath, "/author/profile", false, "获取作者公开资料", nil),
			NewRouteSpec("bilibili", http.MethodGet, BilibiliModuleBasePath, "/author/stats", false, "获取作者统计", nil),
			NewRouteSpec("bilibili", http.MethodGet, BilibiliModuleBasePath, "/author/videos", false, "获取作者公开视频", nil),
			NewRouteSpec("bilibili", http.MethodGet, BilibiliModuleBasePath, "/author/homepage", false, "获取作者空间首屏", nil),
		}
	}
	return []RouteSpec{
		NewRouteSpec("bilibili", http.MethodGet, BilibiliModuleBasePath, "/author/profile", false, "获取作者公开资料", handler.GetProfile),
		NewRouteSpec("bilibili", http.MethodGet, BilibiliModuleBasePath, "/author/stats", false, "获取作者统计", handler.GetStats),
		NewRouteSpec("bilibili", http.MethodGet, BilibiliModuleBasePath, "/author/videos", false, "获取作者公开视频", handler.GetVideos),
		NewRouteSpec("bilibili", http.MethodGet, BilibiliModuleBasePath, "/author/homepage", false, "获取作者空间首屏", handler.GetHomepage),
	}
}
