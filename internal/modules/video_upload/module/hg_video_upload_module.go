package VideoUploadModulePackage

import (
	HGHandlerPackage "MLC_GO/internal/handler"
	PersistenceSQLPackage "MLC_GO/internal/infrastructure/persistence/mysql"
	PersistenceRedisPackage "MLC_GO/internal/infrastructure/persistence/redis"
	HGMiddlewareGroupPackage "MLC_GO/internal/pkg/middleware/middleware_group"
	VideoUploadHandlerPackage "MLC_GO/internal/modules/video_upload/handler"
	"net/http"
)

// Module 实现 HGModule 接口，用于把视频投稿能力挂载到根路由。
type Module struct {
	handler *VideoUploadHandlerPackage.Handler
}

// NewModule 创建视频投稿模块实例。
func NewModule(handler *VideoUploadHandlerPackage.Handler) *Module {
	return &Module{handler: handler}
}

// Name 返回模块名称，用于日志和路由清单。
func (m *Module) Name() string {
	return "video_upload"
}

// BasePath 返回模块 API 前缀。
func (m *Module) BasePath() string {
	return HGMiddlewareGroupPackage.VideoUploadModuleBasePath
}

// Handler 返回带 API Guard、JWT 鉴权和基础中间件的视频投稿路由组。
func (m *Module) Handler() http.Handler {
	return HGMiddlewareGroupPackage.NewVideoUploadRouteGroup(m.handler)
}

// RegisterModules 注册视频投稿模块，内部完成依赖创建并写入全局模块注册表。
func RegisterModules(redisService *PersistenceRedisPackage.RedisService, sqlManager *PersistenceSQLPackage.HGSQLManager) {
	components := NewModuleComponents(ModuleDeps{RedisService: redisService, SQLManager: sqlManager})
	HGHandlerPackage.RegisterModule(NewModule(components.Handler))
}
