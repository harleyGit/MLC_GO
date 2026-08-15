package BilibiliModulePackage

import (
	HGHandlerPackage "MLC_GO/internal/handler"
	BilibiliHandlerPackage "MLC_GO/internal/modules/bilibili/handler"
	HGRouterPackage "MLC_GO/internal/pkg/hg_router"
	PersistenceSQLPackage "MLC_GO/internal/pkg/mysql"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"net/http"
)

// Module 将 Bilibili 作者空间能力挂载到根路由。
type Module struct {
	handler *BilibiliHandlerPackage.Handler
}

// NewModule 创建作者空间模块。
func NewModule(handler *BilibiliHandlerPackage.Handler) *Module { return &Module{handler: handler} }

// Name 返回模块名称。
func (m *Module) Name() string { return "bilibili" }

// BasePath 返回模块 API 前缀。
func (m *Module) BasePath() string { return HGRouterPackage.BilibiliModuleBasePath }

// Handler 返回作者空间路由组。
func (m *Module) Handler() http.Handler { return HGRouterPackage.NewBilibiliRouteGroup(m.handler) }

// RegisterModules 注册作者空间模块。
func RegisterModules(redisService *PersistenceRedisPackage.RedisService, sqlManager *PersistenceSQLPackage.HGSQLManager) {
	components := NewModuleComponents(ModuleDeps{RedisService: redisService, SQLManager: sqlManager})
	HGHandlerPackage.RegisterModule(NewModule(components.Handler))
}
