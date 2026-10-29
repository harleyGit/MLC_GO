package OpsModulePackage

import (
	CoinRepositoryPackage "MLC_GO/internal/modules/coin/repository"
	CoinServicePackage "MLC_GO/internal/modules/coin/service"
	CrawlerLeasePackage "MLC_GO/internal/modules/crawler/lease"
	CrawlerRepositoryPackage "MLC_GO/internal/modules/crawler/repository"
	CrawlerServicePackage "MLC_GO/internal/modules/crawler/service"
	IAMPackage "MLC_GO/internal/modules/iam"
	OpsCachePackage "MLC_GO/internal/modules/ops/cache"
	OpsHandlerPackage "MLC_GO/internal/modules/ops/handler"
	OpsRepositoryPackage "MLC_GO/internal/modules/ops/repository"
	OpsServicePackage "MLC_GO/internal/modules/ops/service"
	OpsTaskPackage "MLC_GO/internal/modules/ops/task"
	VideoInteractionCachePackage "MLC_GO/internal/modules/video_interaction/cache"
	VideoUploadCachePackage "MLC_GO/internal/modules/video_upload/cache"
	ConfigPackage "MLC_GO/internal/pkg/config"
	PersistenceSQLPackage "MLC_GO/internal/pkg/mysql"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
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
	Repo         *OpsRepositoryPackage.Repository        // 运维权威存储。
	Cache        *OpsCachePackage.Cache                  // 缓存与限流。
	TaskPub      OpsTaskPackage.Publisher                // 后台任务发布器。
	Operational  *OpsServicePackage.HGOperationalService // 资产操作服务。
	Service      *OpsServicePackage.Service              // 运维业务服务。
	Handler      *OpsHandlerPackage.Handler              // HTTP 边界。
	CrawlerRepo  *CrawlerRepositoryPackage.Repository    // 采集任务仓储。
	CrawlerTasks *CrawlerServicePackage.HGTaskService    // 采集任务服务。
	Authorizer   *IAMPackage.HGAssetAuthorizer           // 复用业务 IAM 实例执行路由授权。
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
	iam := IAMPackage.NewHGService(IAMPackage.NewHGRepository(deps.SQLManager.GetSQLDB()).WithHGPermissionCache(deps.RedisService))
	iamAuthorizer := IAMPackage.NewHGAssetAuthorizer(iam)
	cache := OpsCachePackage.NewCache(deps.RedisService)
	taskPub := OpsTaskPackage.NewMemoryPublisher()
	coinRepository := CoinRepositoryPackage.NewHGRepository(deps.SQLManager.GetSQLDB(), "mlc.domain.events")
	operational := OpsServicePackage.NewHGOperationalService(OpsServicePackage.HGOperationalDeps{
		Authorizer: iamAuthorizer, CoinAssets: CoinServicePackage.NewHGService(coinRepository), CoinQueries: coinRepository,
		ProjectionCheckpoints: VideoInteractionCachePackage.NewCache(deps.RedisService),
		Audit:                 repo, RateLimiter: cache, Corrections: repo, UserLookup: repo,
	})
	service := OpsServicePackage.NewService(repo, cache, taskPub, operational)
	crawlerConfig, err := ConfigPackage.GetCrawlerTaskConfig()
	if err != nil {
		panic(err)
	}
	policy, err := CrawlerServicePackage.NewHGTargetPolicy(crawlerConfig.AllowedHosts, crawlerConfig.AllowHTTP)
	if err != nil {
		panic(err)
	}
	httpService, err := CrawlerServicePackage.NewHGSafeHTTPService(policy, crawlerConfig.DefaultUserAgent)
	if err != nil {
		panic(err)
	}
	crawlerRepo := CrawlerRepositoryPackage.NewRepository(deps.SQLManager.GetSQLDB())
	crawlerRepo.WithHGVideoPipeline(ConfigPackage.HGCrawlerVideoPipelineEnabled())
	externalStore := CrawlerServicePackage.NewHGExternalContentStore(crawlerRepo, VideoUploadCachePackage.NewCache(deps.RedisService))
	crawlerTasks, err := CrawlerServicePackage.NewHGTaskService(crawlerRepo, httpService, CrawlerLeasePackage.NewHGRedisTaskLease(deps.RedisService), crawlerConfig.LeaseGrace, externalStore)
	if err != nil {
		panic(err)
	}
	handler := OpsHandlerPackage.NewHandler(service, CrawlerServicePackage.NewHGDebugService(httpService)).WithCrawlerTasks(crawlerTasks, iamAuthorizer)

	return &ModuleComponents{
		Repo:         repo,
		Cache:        cache,
		TaskPub:      taskPub,
		Operational:  operational,
		Service:      service,
		Handler:      handler,
		CrawlerRepo:  crawlerRepo,
		CrawlerTasks: crawlerTasks,
		Authorizer:   iamAuthorizer,
	}
}
