package HGRouterPackage

import (
	VideoCommentHandlerPackage "MLC_GO/internal/modules/video_comment/handler"
	"net/http"
)

// videoCommentRoutes 返回认证视频评论模块的完整固定路由定义。
func videoCommentRoutes(handler *VideoCommentHandlerPackage.Handler) []RouteSpec {
	var create, list, replies, reaction, image, deleteComment http.HandlerFunc
	if handler != nil {
		create, list, replies, reaction, image, deleteComment = handler.Create, handler.List, handler.Replies, handler.Reaction, handler.Image, handler.Delete
	}
	return []RouteSpec{
		NewRouteSpec("video_comment", http.MethodPost, VideoCommentModuleBasePath, "/create", true, "创建视频评论", create),
		NewRouteSpec("video_comment", http.MethodGet, VideoCommentModuleBasePath, "/list", true, "获取视频评论", list),
		NewRouteSpec("video_comment", http.MethodGet, VideoCommentModuleBasePath, "/replies", true, "获取视频评论回复", replies),
		NewRouteSpec("video_comment", http.MethodPost, VideoCommentModuleBasePath, "/reaction", true, "设置视频评论反应", reaction),
		NewRouteSpec("video_comment", http.MethodPost, VideoCommentModuleBasePath, "/image", true, "上传视频评论图片", image),
		NewRouteSpec("video_comment", http.MethodPost, VideoCommentModuleBasePath, "/delete", true, "删除视频评论", deleteComment),
	}
}
