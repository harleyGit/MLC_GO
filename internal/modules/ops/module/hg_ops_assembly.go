package OpsModulePackage

import (
	PersistenceSQLPackage "MLC_GO/internal/infrastructure/persistence/mysql"
	PersistenceRedisPackage "MLC_GO/internal/infrastructure/persistence/redis"
	OpsCachePackage "MLC_GO/internal/modules/ops/cache"
	OpsHandlerPackage "MLC_GO/internal/modules/ops/handler"
	OpsRepositoryPackage "MLC_GO/internal/modules/ops/repository"
	OpsServicePackage "MLC_GO/internal/modules/ops/service"
	OpsTaskPackage "MLC_GO/internal/modules/ops/task"
)

// ModuleDeps 声明 ops 模块依赖的基础设施。
// MySQL 存储运维管理数据，Redis 负责缓存和分布式锁。
type ModuleDeps struct {
	RedisService *PersistenceRedisPackage.RedisService
	SQLManager   *PersistenceSQLPackage.HGSQLManager
}

// ModuleComponents 保存模块内部组装出的 repo/service/handler。
// 统一放在 assembly 中创建，避免 handler 或 service 自己 new 下层依赖。
type ModuleComponents struct {
	Repo    *OpsRepositoryPackage.Repository
	Cache   *OpsCachePackage.Cache
	TaskPub OpsTaskPackage.Publisher
	Service *OpsServicePackage.Service
	Handler *OpsHandlerPackage.Handler
}

// NewModuleComponents 负责组装 ops 模块依赖链。
// 构建顺序保持 repo -> service -> handler，和 video_upload 模块的装配方式一致。
func NewModuleComponents(deps ModuleDeps) *ModuleComponents {
	if deps.SQLManager == nil {
		panic("ops module requires sql manager")
	}
	if deps.RedisService == nil {
		panic("ops module requires redis service")
	}

	repo := OpsRepositoryPackage.NewRepository(deps.SQLManager.GetSQLDB())
	cache := OpsCachePackage.NewCache(deps.RedisService)
	taskPub := OpsTaskPackage.NewMemoryPublisher()
	service := OpsServicePackage.NewService(repo, cache, taskPub)
	handler := OpsHandlerPackage.NewHandler(service)

	return &ModuleComponents{
		Repo:    repo,
		Cache:   cache,
		TaskPub: taskPub,
		Service: service,
		Handler: handler,
	}
}