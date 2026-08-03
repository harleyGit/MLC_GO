package VideoCommentModulePackage

import (
	HGHandlerPackage "MLC_GO/internal/handler"
	VideoCommentHandlerPackage "MLC_GO/internal/modules/video_comment/handler"
	VideoCommentRepositoryPackage "MLC_GO/internal/modules/video_comment/repository"
	VideoCommentServicePackage "MLC_GO/internal/modules/video_comment/service"
	HGRouterPackage "MLC_GO/internal/pkg/hg_router"
	PersistenceSQLPackage "MLC_GO/internal/pkg/mysql"
	"net/http"
)

// Module 向统一模块注册器暴露视频评论路由组。
type Module struct {
	handler *VideoCommentHandlerPackage.Handler
}

// Name 返回统一模块注册器使用的模块名。
func (m *Module) Name() string { return "video_comment" }

// BasePath 返回视频评论 API 的统一基础路径。
func (m *Module) BasePath() string { return HGRouterPackage.VideoCommentModuleBasePath }

// Handler 返回已应用认证和方法白名单的视频评论路由组。
func (m *Module) Handler() http.Handler {
	return HGRouterPackage.NewVideoCommentRouteGroup(m.handler)
}

// RegisterModules 组装并注册同步 MySQL 视频评论模块。
func RegisterModules(sqlManager *PersistenceSQLPackage.HGSQLManager) {
	repo := VideoCommentRepositoryPackage.NewRepository(sqlManager.GetSQLDB())
	service := VideoCommentServicePackage.NewService(repo)
	HGHandlerPackage.RegisterModule(&Module{handler: VideoCommentHandlerPackage.NewHandler(service)})
}
