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

// UserModule 实现 Module 接口，需要登录的用户路由。
type UserModule struct {
	handler *UserHandlerPackage.UserHandler
}

// NewUserModule 创建用户模块实例。
func NewUserModule(handler *UserHandlerPackage.UserHandler) *UserModule {
	return &UserModule{handler: handler}
}

// Name 返回模块名称，用于日志和路由清单。
func (m *UserModule) Name() string {
	return "user"
}

// BasePath 返回模块的 API 前缀路径。
func (m *UserModule) BasePath() string {
	return HGMiddlewareGroupPackage.UserProfileModuleBasePath
}

// Handler 返回模块的 HTTP Handler，包含 JWT 鉴权中间件。
func (m *UserModule) Handler() http.Handler {
	return HGMiddlewareGroupPackage.NewUserRouteInterceptorGroup(m.handler)
}

// AuthModule 实现 Module 接口，公开的认证路由。
type AuthModule struct {
	handler *UserHandlerPackage.UserHandler
}

// NewAuthModule 创建认证模块实例。
func NewAuthModule(handler *UserHandlerPackage.UserHandler) *AuthModule {
	return &AuthModule{handler: handler}
}

func (m *AuthModule) Name() string {
	return "auth"
}

func (m *AuthModule) BasePath() string {
	return HGMiddlewareGroupPackage.AuthModuleBasePath
}

func (m *AuthModule) Handler() http.Handler {
	return HGMiddlewareGroupPackage.NewAuthRouteInterceptorGroup(m.handler)
}

// RegisterModules 注册用户相关模块，内部创建所有依赖。
func RegisterModules(redisService *PersistenceRedisPackage.RedisService, sqlManager *PersistenceSQLPackage.HGSQLManager, smsSender HGSMSPackage.HGSender) {
	handler := UserHandlerPackage.NewUserHandler(UserHandlerPackage.UserHandlerDeps{
		RedisService: redisService,
		SQLManager:   sqlManager,
		SMSSender:    smsSender,
	})

	HGHandlerPackage.RegisterModule(
		NewAuthModule(handler),
		NewUserModule(handler),
	)
}
