package VideoUploadModulePackage

import (
	PersistenceSQLPackage "MLC_GO/internal/infrastructure/persistence/mysql"
	PersistenceRedisPackage "MLC_GO/internal/infrastructure/persistence/redis"
	VideoUploadCachePackage "MLC_GO/internal/modules/video_upload/cache"
	VideoUploadHandlerPackage "MLC_GO/internal/modules/video_upload/handler"
	VideoUploadRepositoryPackage "MLC_GO/internal/modules/video_upload/repository"
	VideoUploadServicePackage "MLC_GO/internal/modules/video_upload/service"
	VideoUploadTaskPackage "MLC_GO/internal/modules/video_upload/task"
)

// ModuleDeps 声明 video_upload 模块依赖的基础设施。
// MySQL 存储投稿元数据，Redis 负责上传会话、限流和幂等，任务发布器负责异步转码/审核调度。
type ModuleDeps struct {
	RedisService *PersistenceRedisPackage.RedisService
	SQLManager   *PersistenceSQLPackage.HGSQLManager
}

// ModuleComponents 保存模块内部组装出的 repo/service/handler。
// 统一放在 assembly 中创建，避免 handler 或 service 自己 new 下层依赖。
type ModuleComponents struct {
	Repo    *VideoUploadRepositoryPackage.Repository
	Cache   *VideoUploadCachePackage.Cache
	TaskPub VideoUploadTaskPackage.Publisher
	Service *VideoUploadServicePackage.Service
	Handler *VideoUploadHandlerPackage.Handler
}

// NewModuleComponents 负责组装 video_upload 模块依赖链。
// 构建顺序保持 repo -> service -> handler，和 user 模块的装配方式一致。
func NewModuleComponents(deps ModuleDeps) *ModuleComponents {
	if deps.SQLManager == nil {
		panic("video upload module requires sql manager")
	}
	if deps.RedisService == nil {
		panic("video upload module requires redis service")
	}

	repo := VideoUploadRepositoryPackage.NewRepository(deps.SQLManager.GetSQLDB())
	cache := VideoUploadCachePackage.NewCache(deps.RedisService)
	taskPub := VideoUploadTaskPackage.NewMemoryPublisher()
	service := VideoUploadServicePackage.NewService(repo, cache, taskPub)
	handler := VideoUploadHandlerPackage.NewHandler(service)

	return &ModuleComponents{
		Repo:    repo,
		Cache:   cache,
		TaskPub: taskPub,
		Service: service,
		Handler: handler,
	}
}
