/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-05-05 17:49:12
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-05-05 21:49:05
 * @FilePath: /MLC_GO/internal/modules/user/module/hg_user_module.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package HGUserModulePackage

import (
	HGMiddlewareGroupPackage "MLC_GO/internal/interfaces/middleware/middleware_group"
	HGSMSPackage "MLC_GO/internal/modules/sms"
	UserHandlerPackage "MLC_GO/internal/modules/user/handler"
	"net/http"

	HGHandlerPackage "MLC_GO/internal/handler"
	PersistenceSQLPackage "MLC_GO/internal/infrastructure/persistence/mysql"
	PersistenceRedisPackage "MLC_GO/internal/infrastructure/persistence/redis"
)

// HGUserModule 实现 HGModule 接口，需要登录的用户路由。
type HGUserModule struct {
	handler *UserHandlerPackage.HGUserHandler
}

// NewUserModule 创建用户模块实例。
func NewUserModule(handler *UserHandlerPackage.HGUserHandler) *HGUserModule {
	return &HGUserModule{handler: handler}
}

// Name 返回模块名称，用于日志和路由清单。
func (m *HGUserModule) Name() string {
	return "user"
}

// BasePath 返回模块的 API 前缀路径。
func (m *HGUserModule) BasePath() string {
	return HGMiddlewareGroupPackage.UserProfileModuleBasePath
}

// Handler 返回模块的 HTTP Handler，包含 JWT 鉴权中间件。
func (m *HGUserModule) Handler() http.Handler {
	return HGMiddlewareGroupPackage.NewUserRouteInterceptorGroup(m.handler)
}

// HGAuthModule 实现 Module 接口，公开的认证路由。
type HGAuthModule struct {
	handler *UserHandlerPackage.HGUserHandler
}

// NewAuthModule 创建认证模块实例。
func NewAuthModule(handler *UserHandlerPackage.HGUserHandler) *HGAuthModule {
	return &HGAuthModule{handler: handler}
}

func (m *HGAuthModule) Name() string {
	return "auth"
}

func (m *HGAuthModule) BasePath() string {
	return HGMiddlewareGroupPackage.AuthModuleBasePath
}

func (m *HGAuthModule) Handler() http.Handler {
	return HGMiddlewareGroupPackage.NewAuthRouteInterceptorGroup(m.handler)
}

// RegisterModules 注册用户相关模块，内部创建所有依赖。
func RegisterModules(redisService *PersistenceRedisPackage.RedisService, sqlManager *PersistenceSQLPackage.HGSQLManager, smsSender HGSMSPackage.HGSender) {
	handler := UserHandlerPackage.NewUserHandler(UserHandlerPackage.HGUserHandlerDeps{
		RedisService: redisService,
		SQLManager:   sqlManager,
		SMSSender:    smsSender,
	})

	HGHandlerPackage.RegisterModule(
		NewAuthModule(handler), // HGAuthModule实现了hg_module.go中的接口
		NewUserModule(handler),
	)
}
