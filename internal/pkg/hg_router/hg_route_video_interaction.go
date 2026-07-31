package HGRouterPackage

import (
	VideoInteractionHandlerPackage "MLC_GO/internal/modules/video_interaction/handler"
	"net/http"
)

func videoInteractionRoutes(handler *VideoInteractionHandlerPackage.Handler) []RouteSpec {
	var state, like, coin, favorite, share, follow http.HandlerFunc
	if handler != nil {
		state, like, coin = handler.State, handler.Like, handler.Coin
		favorite, share, follow = handler.Favorite, handler.Share, handler.Follow
	}
	return []RouteSpec{
		NewRouteSpec("video_interaction", http.MethodGet, VideoInteractionModuleBasePath, "/state", true, "获取视频互动状态和计数", state),
		NewRouteSpec("video_interaction", http.MethodPost, VideoInteractionModuleBasePath, "/like", true, "点赞或取消点赞", like),
		NewRouteSpec("video_interaction", http.MethodPost, VideoInteractionModuleBasePath, "/coin", true, "为视频投币", coin),
		NewRouteSpec("video_interaction", http.MethodPost, VideoInteractionModuleBasePath, "/favorite", true, "收藏或取消收藏", favorite),
		NewRouteSpec("video_interaction", http.MethodPost, VideoInteractionModuleBasePath, "/share", true, "记录视频分享", share),
		NewRouteSpec("video_interaction", http.MethodPost, VideoInteractionModuleBasePath, "/follow", true, "关注或取消关注作者", follow),
	}
}
