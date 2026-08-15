package VideoRecommendModulePackage

import (
	VideoRecommendCachePackage "MLC_GO/internal/modules/video_recommend/cache"
	VideoRecommendHandlerPackage "MLC_GO/internal/modules/video_recommend/handler"
	VideoRecommendRepositoryPackage "MLC_GO/internal/modules/video_recommend/repository"
	VideoRecommendServicePackage "MLC_GO/internal/modules/video_recommend/service"
	ConfigPackage "MLC_GO/internal/pkg/config"
	PersistenceSQLPackage "MLC_GO/internal/pkg/mysql"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
)

// ModuleDeps 声明推荐模块依赖的共享 Redis/MySQL 连接池。
type ModuleDeps struct {
	RedisService *PersistenceRedisPackage.RedisService
	SQLManager   *PersistenceSQLPackage.HGSQLManager
}

// ModuleComponents 保存推荐模块依赖链。
type ModuleComponents struct {
	Repo    *VideoRecommendRepositoryPackage.Repository
	Cache   *VideoRecommendCachePackage.Cache
	Service *VideoRecommendServicePackage.Service
	Handler *VideoRecommendHandlerPackage.Handler
}

// NewModuleComponents 按 repository/cache -> service -> handler 组装推荐模块。
func NewModuleComponents(deps ModuleDeps) (*ModuleComponents, error) {
	config, err := ConfigPackage.GetVideoRecommendConfig()
	if err != nil {
		return nil, err
	}
	repo := VideoRecommendRepositoryPackage.NewRepository(deps.SQLManager.GetSQLDB())
	cache := VideoRecommendCachePackage.NewCache(deps.RedisService, config.RedisGeneration, config.RedisShardCount)
	service := VideoRecommendServicePackage.NewService(cache, repo, config.RedisGeneration)
	return &ModuleComponents{Repo: repo, Cache: cache, Service: service, Handler: VideoRecommendHandlerPackage.NewHandler(service)}, nil
}
