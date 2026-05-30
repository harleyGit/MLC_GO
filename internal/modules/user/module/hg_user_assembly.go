package HGUserModulePackage

import (
	PersistenceSQLPackage "MLC_GO/internal/pkg/mysql"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	HGSMSPackage "MLC_GO/internal/modules/sms"
	usercache "MLC_GO/internal/modules/user/cache"
	UserHandlerPackage "MLC_GO/internal/modules/user/handler"
	userrepository "MLC_GO/internal/modules/user/repository"
	UserServicePackage "MLC_GO/internal/modules/user/service"
)

// UserModuleDeps 是 user 模块对外部基础设施的依赖声明。
//
// 大厂常用做法是：模块入口只接收基础设施对象，不在 handler/service 中临时创建依赖。
// 这样可以明确依赖边界，避免业务代码到处 new DB、new Redis，也方便测试时替换依赖。
type UserModuleDeps struct {
	RedisService *PersistenceRedisPackage.RedisService
	SQLManager   *PersistenceSQLPackage.HGSQLManager
	SMSSender    HGSMSPackage.HGSender
}

// UserModuleComponents 保存 user 模块内部组装出的组件。
//
// 该结构的价值：
// 1. module 层统一管理 repo/cache/service/handler 的创建顺序。
// 2. handler 不再知道 repository/cache 的存在，避免 HTTP 层污染业务和数据层。
// 3. 后续增加依赖时，只改 assembly，不需要到多个 handler 构造函数里散改。
type UserModuleComponents struct {
	UserRepo     *userrepository.UserRepo
	UserCache    *usercache.HGUserCache
	CodeCache    *usercache.HGCodeCache
	UserService  *UserServicePackage.UserService
	TokenService *UserServicePackage.HGAuthService
	AvatarSvc    *UserServicePackage.AvatarService
	Handler      *UserHandlerPackage.HGUserHandler
}

// NewUserModuleComponents 负责组装 user 模块内部依赖。
//
// 推荐规范：
// 1. repository 只接数据库连接，负责 SQL 访问。
// 2. cache 只接 Redis service，负责缓存 key/value 访问。
// 3. service 只接 repo/cache/基础服务，负责编排业务流程。
// 4. handler 只接 service，负责 HTTP 参数、响应和错误码。
func NewUserModuleComponents(deps UserModuleDeps) *UserModuleComponents {
	if deps.RedisService == nil {
		panic("user module requires redis service")
	}
	if deps.SQLManager == nil {
		panic("user module requires sql manager")
	}

	db := deps.SQLManager.GetSQLDB()
	userRepo := userrepository.NewUserRepo(db)
	userCache := usercache.NewUserCache(deps.RedisService)
	codeCache := usercache.NewCodeCache(deps.RedisService)

	userService := UserServicePackage.NewUserService(userRepo, userCache, deps.RedisService)
	tokenService := UserServicePackage.NewAuthService(userRepo, codeCache, deps.RedisService)
	avatarSvc := UserServicePackage.NewAvatarService(userService)

	handler := UserHandlerPackage.NewUserHandler(UserHandlerPackage.HGUserHandlerDeps{
		UserService:  userService,
		TokenService: tokenService,
		AvatarSvc:    avatarSvc,
		SMSSender:    deps.SMSSender,
	})

	return &UserModuleComponents{
		UserRepo:     userRepo,
		UserCache:    userCache,
		CodeCache:    codeCache,
		UserService:  userService,
		TokenService: tokenService,
		AvatarSvc:    avatarSvc,
		Handler:      handler,
	}
}
