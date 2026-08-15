package VideoRecommendModulePackage

import (
	HGHandlerPackage "MLC_GO/internal/handler"
	VideoRecommendHandlerPackage "MLC_GO/internal/modules/video_recommend/handler"
	HGMiddlewareGroupPackage "MLC_GO/internal/pkg/hg_router"
	PersistenceSQLPackage "MLC_GO/internal/pkg/mysql"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"fmt"
	"net/http"
)

// Module 实现 HGModule，把视频推荐流挂载到根路由。
type Module struct {
	handler *VideoRecommendHandlerPackage.Handler
}

// NewModule 创建视频推荐模块。
func NewModule(handler *VideoRecommendHandlerPackage.Handler) *Module {
	return &Module{handler: handler}
}

// Name 返回模块名称。
func (m *Module) Name() string { return "video_recommend" }

// BasePath 返回推荐 API 前缀。
func (m *Module) BasePath() string { return HGMiddlewareGroupPackage.VideoRecommendModuleBasePath }

// Handler 返回带 API Guard、JWT 和基础响应中间件的路由组。
func (m *Module) Handler() http.Handler {
	return HGMiddlewareGroupPackage.NewVideoRecommendRouteGroup(m.handler)
}

// RegisterModules 注册视频推荐模块。
func RegisterModules(redisService *PersistenceRedisPackage.RedisService, sqlManager *PersistenceSQLPackage.HGSQLManager) error {
	if redisService == nil || redisService.Client() == nil || sqlManager == nil || sqlManager.GetSQLDB() == nil {
		return fmt.Errorf("video recommend module dependencies cannot be nil")
	}
	components, err := NewModuleComponents(ModuleDeps{RedisService: redisService, SQLManager: sqlManager})
	if err != nil {
		return err
	}
	HGHandlerPackage.RegisterModule(NewModule(components.Handler))
	return nil
}
