package HGRouterPackage

import (
	VideoDanmakuHandlerPackage "MLC_GO/internal/modules/video_danmaku/handler"
	"net/http"
)

// videoDanmakuRoutes 返回弹幕 HTTP 路由和由独立 gnet 端口提供的 WebSocket 路由元数据。
func videoDanmakuRoutes(handler *VideoDanmakuHandlerPackage.Handler) []RouteSpec {
	var create, list, ticket http.HandlerFunc
	if handler != nil {
		create, list, ticket = handler.Create, handler.List, handler.Ticket
	}
	return []RouteSpec{
		NewRouteSpec("video_danmaku", http.MethodPost, VideoDanmakuModuleBasePath, "/create", true, "创建视频弹幕", create),
		NewRouteSpec("video_danmaku", http.MethodGet, VideoDanmakuModuleBasePath, "/list", true, "按播放时间窗获取视频弹幕", list),
		NewRouteSpec("video_danmaku", http.MethodPost, VideoDanmakuModuleBasePath, "/ticket", true, "获取单次 WebSocket 票据", ticket),
		// Handler=nil 使标准 HTTP mux 不注册该路径，但路由清单仍对前端声明公开协议。
		NewRouteSpec("video_danmaku", http.MethodGet, VideoDanmakuModuleBasePath, "/ws", true, "视频弹幕 WebSocket", nil),
	}
}
