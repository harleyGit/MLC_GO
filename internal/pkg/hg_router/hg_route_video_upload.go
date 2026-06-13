package HGRouterPackage

import (
	VideoUploadHandlerPackage "MLC_GO/internal/modules/video_upload/handler"
	"net/http"
)

// videoUploadRoutes 返回 video_upload 模块完整路由定义。
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
