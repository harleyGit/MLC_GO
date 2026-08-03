package HGRouterPackage

import (
	VideoCommentHandlerPackage "MLC_GO/internal/modules/video_comment/handler"
	"net/http"
)

// videoCommentRoutes 返回认证视频评论模块的完整固定路由定义。
func videoCommentRoutes(handler *VideoCommentHandlerPackage.Handler) []RouteSpec {
	var create, list, deleteComment http.HandlerFunc
	if handler != nil {
		create, list, deleteComment = handler.Create, handler.List, handler.Delete
	}
	return []RouteSpec{
		NewRouteSpec("video_comment", http.MethodPost, VideoCommentModuleBasePath, "/create", true, "创建视频评论", create),
		NewRouteSpec("video_comment", http.MethodGet, VideoCommentModuleBasePath, "/list", true, "获取视频评论", list),
		NewRouteSpec("video_comment", http.MethodPost, VideoCommentModuleBasePath, "/delete", true, "删除视频评论", deleteComment),
	}
}
