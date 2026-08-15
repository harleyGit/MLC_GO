package BilibiliModulePackage

import (
	BilibiliCachePackage "MLC_GO/internal/modules/bilibili/cache"
	BilibiliHandlerPackage "MLC_GO/internal/modules/bilibili/handler"
	BilibiliRepositoryPackage "MLC_GO/internal/modules/bilibili/repository"
	BilibiliServicePackage "MLC_GO/internal/modules/bilibili/service"
	PersistenceSQLPackage "MLC_GO/internal/pkg/mysql"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
)

// ModuleDeps 声明作者空间模块所需的 MySQL 与 Redis 依赖。
type ModuleDeps struct {
	RedisService *PersistenceRedisPackage.RedisService
	SQLManager   *PersistenceSQLPackage.HGSQLManager
}

// ModuleComponents 保存作者空间模块组装出的组件。
type ModuleComponents struct {
	Repo    *BilibiliRepositoryPackage.Repository
	Cache   *BilibiliCachePackage.Cache
	Service *BilibiliServicePackage.Service
	Handler *BilibiliHandlerPackage.Handler
}

// NewModuleComponents 按 repo -> cache -> service -> handler 组装依赖。
func NewModuleComponents(deps ModuleDeps) *ModuleComponents {
	if deps.SQLManager == nil || deps.RedisService == nil {
		panic("bilibili author module requires sql manager and redis service")
	}
	repo := BilibiliRepositoryPackage.NewRepository(deps.SQLManager.GetSQLDB())
	cache := BilibiliCachePackage.NewCache(deps.RedisService)
	service := BilibiliServicePackage.NewService(repo, cache)
	handler := BilibiliHandlerPackage.NewHandler(service)
	return &ModuleComponents{Repo: repo, Cache: cache, Service: service, Handler: handler}
}
