package OpsModulePackage

import (
	HGHandlerPackage "MLC_GO/internal/handler"
	OpsHandlerPackage "MLC_GO/internal/modules/ops/handler"
	HGMiddlewareGroupPackage "MLC_GO/internal/pkg/hg_router"
	PersistenceSQLPackage "MLC_GO/internal/pkg/mysql"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"net/http"
)

// Module 实现 HGModule 接口，用于把运维管理能力挂载到根路由。
type Module struct {
	handler      *OpsHandlerPackage.Handler                 // 运维 HTTP 入口。
	hgAuthorizer HGMiddlewareGroupPackage.HGRouteAuthorizer // JWT 后的 IAM 路由权限检查器。
}

// NewModule 创建运维管理模块实例。
func NewModule(handler *OpsHandlerPackage.Handler, hgAuthorizers ...HGMiddlewareGroupPackage.HGRouteAuthorizer) *Module {
	hgModule := &Module{handler: handler}
	if len(hgAuthorizers) > 0 {
		hgModule.hgAuthorizer = hgAuthorizers[0]
	}
	return hgModule
}

// Name 返回模块名称，用于日志和路由清单。
func (m *Module) Name() string {
	return "ops"
}

// BasePath 返回模块 API 前缀。
func (m *Module) BasePath() string {
	return HGMiddlewareGroupPackage.OpsModuleBasePath
}

// Handler 返回带 API Guard、JWT 鉴权和基础中间件的运维管理路由组。
func (m *Module) Handler() http.Handler {
	return HGMiddlewareGroupPackage.NewOpsRouteGroup(m.handler, m.hgAuthorizer)
}

// RegisterModules 注册运维管理模块，内部完成依赖创建并写入全局模块注册表。
func RegisterModules(redisService *PersistenceRedisPackage.RedisService, sqlManager *PersistenceSQLPackage.HGSQLManager) *ModuleComponents {
	components := NewModuleComponents(ModuleDeps{RedisService: redisService, SQLManager: sqlManager})
	HGHandlerPackage.RegisterModule(NewModule(components.Handler, components.Authorizer))
	return components
}
