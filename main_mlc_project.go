package main

import (
	StatisticConsumerPackage "MLC_GO/internal/consumer/statistic"
	HGHandlerPackage "MLC_GO/internal/handler"
	BilibiliModulePackage "MLC_GO/internal/modules/bilibili/module"
	CoinRepositoryPackage "MLC_GO/internal/modules/coin/repository"
	CoinTaskPackage "MLC_GO/internal/modules/coin/task"
	CrawlerRepositoryPackage "MLC_GO/internal/modules/crawler/repository"
	CrawlerRuntimePackage "MLC_GO/internal/modules/crawler/runtime"
	CrawlerServicePackage "MLC_GO/internal/modules/crawler/service"
	CrawlerSpiderPackage "MLC_GO/internal/modules/crawler/spider"
	OpsModulePackage "MLC_GO/internal/modules/ops/module"
	OpsRepositoryPackage "MLC_GO/internal/modules/ops/repository"
	OpsTaskPackage "MLC_GO/internal/modules/ops/task"
	HGTestHandlerPackage "MLC_GO/internal/modules/test/handler"
	HGUserModulePackage "MLC_GO/internal/modules/user/module"
	VideoCommentModulePackage "MLC_GO/internal/modules/video_comment/module"
	VideoCommentTaskPackage "MLC_GO/internal/modules/video_comment/task"
	VideoDanmakuModulePackage "MLC_GO/internal/modules/video_danmaku/module"
	VideoDanmakuRealtimePackage "MLC_GO/internal/modules/video_danmaku/realtime"
	VideoInteractionCachePackage "MLC_GO/internal/modules/video_interaction/cache"
	VideoInteractionModulePackage "MLC_GO/internal/modules/video_interaction/module"
	VideoInteractionRepositoryPackage "MLC_GO/internal/modules/video_interaction/repository"
	VideoInteractionTaskPackage "MLC_GO/internal/modules/video_interaction/task"
	VideoRecommendModulePackage "MLC_GO/internal/modules/video_recommend/module"
	VideoUploadCachePackage "MLC_GO/internal/modules/video_upload/cache"
	VideoUploadModulePackage "MLC_GO/internal/modules/video_upload/module"
	ConfigPackage "MLC_GO/internal/pkg/config"
	HGMiddlewareGroupPackage "MLC_GO/internal/pkg/hg_router"
	HGKafkaPackage "MLC_GO/internal/pkg/kafka"
	"MLC_GO/internal/pkg/logHG"
	HGLoggerPackage "MLC_GO/internal/pkg/logger"
	HGMiddlewarePackage "MLC_GO/internal/pkg/middleware"
	PersistenceSQLPackage "MLC_GO/internal/pkg/mysql"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const (
	// mlcServerReadHeaderTimeout 限制客户端发送请求头的最长时间。
	// 防止慢连接/慢请求头长期占用 goroutine 和文件描述符，是公网 HTTP 服务的基础防护。
	mlcServerReadHeaderTimeout = 2 * time.Second
	// mlcServerReadTimeout 限制读取完整请求（含 body）的最长时间。
	// 视频上传会读取较大的 multipart body，不能沿用普通 JSON API 的秒级超时。
	mlcServerReadTimeout = 15 * time.Minute
	// mlcServerWriteTimeout 限制响应写出时间，给上传完成后的写库和响应保留窗口。
	mlcServerWriteTimeout = 15 * time.Minute
	// mlcServerIdleTimeout 控制 keep-alive 空闲连接保留时间，兼顾连接复用和资源释放。
	mlcServerIdleTimeout = 60 * time.Second
	// mlcServerMaxHeaderBytes 限制请求头大小，防止超大 header 造成内存放大和解析成本飙升。
	mlcServerMaxHeaderBytes = 1 << 20
	// infraInitTimeout 限制启动期依赖初始化时间，避免 Redis/MySQL 异常时进程卡死。
	infraInitTimeout = 5 * time.Second
	// mlcServerShutdownTimeout 是收到退出信号后的优雅关闭窗口。
	// 在该时间内 HTTP server 停止接收新请求，并等待正在处理的请求尽量完成。
	mlcServerShutdownTimeout = 10 * time.Second
	// 管理接口只返回小响应，使用更短超时快速释放异常监控连接。
	mlcManagementReadTimeout  = 5 * time.Second
	mlcManagementWriteTimeout = 5 * time.Second
)

// MLCApplication 持有服务运行期依赖，统一管理启动和优雅关闭。
//
// 这样做的原因：
// 1. HTTP Server、Redis、MySQL、Logger 都有自己的生命周期，分散在 main 里容易漏关。
// 2. 容器/K8s 发布时会发送 SIGTERM，应用需要先停止接新流量，再释放连接池。
// 3. 把资源放到一个应用对象里，启动失败和关闭失败都能集中处理，便于维护和测试。
type MLCApplication struct {
	server                  *http.Server
	managementServer        *http.Server
	redisService            *PersistenceRedisPackage.RedisService
	sqlManager              *PersistenceSQLPackage.HGSQLManager
	kafkaCloser             kafkaCloser
	kafkaRuntime            kafkaReadyChecker
	interactionReprojector  *VideoInteractionTaskPackage.HGReprojector
	coinJobs                *CoinTaskPackage.HGJobs
	correctionRecovery      *OpsTaskPackage.HGCorrectionRecovery
	videoCommentMaintenance *VideoCommentTaskPackage.HGVideoCommentMaintenance
	videoDanmakuRealtime    *VideoDanmakuRealtimePackage.Server
	crawlerManager          *CrawlerSpiderPackage.HGManager
	crawlerTaskScheduler    *CrawlerRuntimePackage.HGTaskScheduler
}

// mlc_main 是 MLC_GO 工程入口，负责配置加载、依赖构建与 HTTP 服务启动。
func mlc_main() {
	logHG.DebugInfo("MLC_GO项目启动中...")

	if err := loadRuntimeConfig(); err != nil {
		logHG.ErrFInfo("MLC_GO工程启动失败: %v", err)
		return
	}

	app, err := buildMLCApplication()
	if err != nil {
		logHG.ErrFInfo("MLC_GO工程启动失败: %v", err)
		return
	}
	defer app.Close()

	logHG.DebugFInfo("HTTP server 开始监听: %s", app.server.Addr)
	logHG.DebugFInfo("Management server 开始监听: %s", app.managementServer.Addr)
	if app.videoDanmakuRealtime != nil {
		logHG.DebugFInfo("Danmaku realtime 开始监听: %s", app.videoDanmakuRealtime.Addr())
	}
	if err := app.Serve(context.Background()); err != nil {
		logHG.ErrFInfo("HTTP server 运行失败: %v", err)
	}
}

// loadRuntimeConfig 负责按当前 SERVER_ENV 加载对应的业务配置文件。
func loadRuntimeConfig() error {
	if err := ConfigPackage.InitRuntimeEnv(); err != nil {
		return err
	}
	env := ConfigPackage.GetEnv()
	if err := ConfigPackage.LoadConfig(string(env)); err != nil {
		return fmt.Errorf("加载配置文件失败, env=%s: %w", env, err)
	}

	logHG.DebugFInfo("当前环境: %s", env)
	return nil
}

// buildMLCServer 负责构建工程运行所需依赖，并组装 HTTP Server。
// 保留这个方法是为了兼容旧测试/旧调用；生产入口使用 buildMLCApplication 才能拿到 Close 能力。
func buildMLCServer() (*http.Server, error) {
	app, err := buildMLCApplication()
	if err != nil {
		return nil, err
	}
	return app.server, nil
}

// buildMLCApplication 它负责把 Redis、MySQL、Kafka、各种业务模块、后台定时任务、爬虫、HTTP 路由、健康检查、监控等全部创建出来，组装成一个完整的 MLCApplication，最后交给 main 去启动
//
// 构建顺序刻意保持为：Logger -> Redis -> MySQL -> Kafka -> 模块注册 -> 根路由 -> Server。
// 这样任何一步失败都可以释放前面已成功初始化的资源，避免启动失败时连接泄漏。
/* https://chatgpt.com/s/p_6a9c0be8e9448191ac4f9e9f71067cbe

                  ┌──────────────┐
                  │     main     │
                  └──────┬───────┘
                         ↓
              buildMLCApplication()
                         │
       ┌─────────────────┼──────────────────┐
       ↓                 ↓                  ↓
  基础设施             业务模块            后台任务
       │                 │                  │
 ┌─────┼─────┐      ┌────┼────┐       ┌─────┼─────┐
 ↓     ↓     ↓      ↓    ↓    ↓       ↓     ↓     ↓
Redis MySQL Kafka  User Video Comment  Reproject Coin Crawler
                    │     │      │
                    └─────┼──────┘
                          ↓
                     Route Catalog
                          ↓
                       rootMux
                          ↓
                     Middleware
                          ↓
                    HTTP Server
                          │
               ┌──────────┴──────────┐
               ↓                     ↓
           :业务端口              :管理端口
               ↓                     ↓
           /api/...           /healthz /readyz
                                 /metrics

*/
/* managementMux、srv、managementServer
return &MLCApplication{……} 这部分代码结构图意思：

                  MLCApplication
                         │
          ┌──────────────┴──────────────┐
          │                             │
     business server              management server
          │                             │
     businessHandler             managementMux
          │                             │
    业务 API 请求                 运维管理请求
          │                             │
     ┌────┴────┐                  ┌─────┴─────┐
     │         │                  │           │
   Redis      SQL              Health      Metrics
     │         │               Check       Prometheus
     │         │
     └────┬────┘
          │
        Kafka
          │
     各种业务组件
*/
func buildMLCApplication() (*MLCApplication, error) {
	// ① 初始化日志系统，保证后续的依赖初始化和模块注册都能有日志输出。
	HGLoggerPackage.Init()

	// 1. 初始化基础设施。
	// ② Redis Redis/MySQL 属于服务 ready 状态的关键依赖；启动期直接校验能尽早暴露配置和网络问题。
	redisService, err := newRedisServiceWithRecover()
	if err != nil {
		// 启动失败，关闭日志系统。
		HGLoggerPackage.CloseLogger()
		return nil, fmt.Errorf("Redis初始化失败: %w", err)
	}

	// 从配置文件 / 环境变量中读取 API Gateway 的配置。
	gatewayConfig, err := ConfigPackage.GetAPIGatewayConfig()
	if err != nil {
		// Redis 已经创建了，不能不管，初始化失败回滚。
		_ = redisService.Close()
		HGLoggerPackage.CloseLogger()
		return nil, fmt.Errorf("API Gateway配置失败: %w", err)
	}
	// ③ API Gateway
	apiGateway, err := HGMiddlewareGroupPackage.NewHGAPIGateway(redisService, gatewayConfig)
	if err != nil {
		_ = redisService.Close()
		HGLoggerPackage.CloseLogger()
		return nil, fmt.Errorf("API Gateway初始化失败: %w", err)
	}
	// ④ MySQL
	sqlManager, err := PersistenceSQLPackage.NewSQLManager()
	if err != nil {
		logHG.ErrFInfo("数据库初始化失败: %v", err)
		_ = redisService.Close()
		HGLoggerPackage.CloseLogger()
		return nil, fmt.Errorf("数据库初始化失败: %w", err)
	}

	// ⑤ Kafka 是启动必需依赖：配置缺失、静态校验失败或 broker Ping 失败都会中止应用构建。
	// Kafka 主要负责：业务事件 -> Kafka -> 异步消费 -> 其他业务, 比如：用户点赞 -> 产生 LikeEvent -> -> Kafka -> 统计服务 -> Redis / ClickHouse
	// kafkaCloser 负责关闭 Kafka； kafkaRuntime 检查Kafka 当前运行状态 / Ready 检查能力。
	kafkaCloser, kafkaRuntime, err := initKafkaWithDependencies(redisService, sqlManager)
	if err != nil {
		logHG.ErrFInfo("Kafka初始化失败: %v", err)
		// Kafka 排在 Redis/MySQL 之后初始化；一旦 Kafka 配置或网络不可用，必须回滚前置资源。
		// 这样可以保证应用启动失败时不会遗留数据库连接池、Redis 连接池或后台日志资源。
		_ = sqlManager.Close()
		_ = redisService.Close()
		HGLoggerPackage.CloseLogger()
		return nil, err
	}
	// 读取配置
	interactionConfig, err := ConfigPackage.GetInteractionReprojectConfig()
	if err != nil {
		if kafkaCloser != nil {
			kafkaCloser()
		}
		_ = sqlManager.Close()
		_ = redisService.Close()
		HGLoggerPackage.CloseLogger()
		return nil, fmt.Errorf("Interaction reproject配置失败: %w", err)
	}
	var interactionReprojector *VideoInteractionTaskPackage.HGReprojector
	if interactionConfig.Enabled {
		hashRanges := make([]VideoInteractionTaskPackage.HGProjectionHashRange, 0, len(interactionConfig.HashRanges))
		for _, hashRange := range interactionConfig.HashRanges {
			hashRanges = append(hashRanges, VideoInteractionTaskPackage.HGProjectionHashRange{Start: hashRange.Start, End: hashRange.End})
		}
		// ⑥ 后台 Worker / Task，创建一个后台任务，可以将 Redis 中的统计数据和数据库里的权威数据出现差异时，进行重新投影 / 修复。
		interactionReprojector, err = VideoInteractionTaskPackage.NewHGReprojector(
			VideoInteractionRepositoryPackage.NewRepository(sqlManager.GetSQLDB()),
			VideoInteractionCachePackage.NewCache(redisService),
			VideoInteractionTaskPackage.HGReprojectConfig{
				Interval: interactionConfig.Interval, Timeout: interactionConfig.Timeout, SafetyLag: interactionConfig.SafetyLag,
				LeaseTTL: interactionConfig.LeaseTTL, PageSize: interactionConfig.PageSize,
				WorkerCount: interactionConfig.WorkerCount, HashRanges: hashRanges,
			},
		)
		if err != nil {
			if kafkaCloser != nil {
				kafkaCloser()
			}
			_ = sqlManager.Close()
			_ = redisService.Close()
			HGLoggerPackage.CloseLogger()
			return nil, fmt.Errorf("Interaction reproject初始化失败: %w", err)
		}
		/** interactionReprojector的使用距离，例如：
		MySQL / ClickHouse
			↓
			权威数据
			↓
			10000 点赞
		Redis
			↓
			9997 点赞

		发现：10000 != 9997

		那么 Reprojector：
			发现差异
			↓
			重新计算
			↓
			修正 Redis

		然后：正式启动后台任务，这里不是 HTTP 请求处理，而是应用启动的时候顺便启动一个后台 Worker。
		*/
		interactionReprojector.Start(context.Background())
	}
	// 读取投币相关任务配置
	coinJobConfig, err := ConfigPackage.GetCoinJobConfig()
	if err != nil {
		if interactionReprojector != nil {
			interactionReprojector.Close()
		}
		if kafkaCloser != nil {
			kafkaCloser()
		}
		_ = sqlManager.Close()
		_ = redisService.Close()
		HGLoggerPackage.CloseLogger()
		return nil, fmt.Errorf("Coin jobs配置失败: %w", err)
	}
	var coinJobs *CoinTaskPackage.HGJobs
	if coinJobConfig.Enabled {
		coinJobs, err = CoinTaskPackage.NewHGJobs(CoinRepositoryPackage.NewHGRepository(sqlManager.GetSQLDB(), "mlc.domain.events"), CoinTaskPackage.HGJobConfig{
			Interval: coinJobConfig.Interval, Timeout: coinJobConfig.Timeout, BatchSize: coinJobConfig.BatchSize,
			ConsolidationBatchSize: coinJobConfig.ConsolidationBatchSize, ConsolidationSourceLimit: coinJobConfig.ConsolidationSourceLimit,
			ConsolidationMaxLotAmount: coinJobConfig.ConsolidationMaxLotAmount,
		}, CoinTaskPackage.NewHGRedisJobLease(redisService))
		if err != nil {
			if interactionReprojector != nil {
				interactionReprojector.Close()
			}
			if kafkaCloser != nil {
				kafkaCloser()
			}
			_ = sqlManager.Close()
			_ = redisService.Close()
			HGLoggerPackage.CloseLogger()
			return nil, fmt.Errorf("Coin jobs初始化失败: %w", err)
		}
		/** 简单理解：Coin jobs 是一个后台任务，负责处理投币相关的业务逻辑，例如：
		用户投币
		↓
		业务事件
		↓
		数据库
		↓
		Coin Jobs
		↓
		定期处理 / 汇总 / 对账
		所以：coinJobs 也是一个后台 Worker
		*/
		coinJobs.Start(context.Background())
	}

	// ⑦ 注册业务模块 2. 注册所有模块（每个模块内部创建自己的 handler）。
	// ClearModules 清空之前注册的模块，用来避免测试或重复构建应用时，全局注册表被 append 出重复模块。
	HGHandlerPackage.ClearModules()
	// 新增模块只需在此处调用 RegisterModules 即可，可以理解成：把用户模块安装到这个应用里面
	HGUserModulePackage.RegisterModules(redisService, sqlManager, nil)
	// 注册上传视频模块时传入 Redis/MySQL 依赖，模块内部创建 Handler 时会用到这些依赖构建 Service 和 Handler。
	VideoUploadModulePackage.RegisterModules(redisService, sqlManager)
	// 视频上传
	BilibiliModulePackage.RegisterModules(redisService, sqlManager)
	if err := VideoRecommendModulePackage.RegisterModules(redisService, sqlManager); err != nil {
		if coinJobs != nil {
			coinJobs.Close()
		}
		if interactionReprojector != nil {
			interactionReprojector.Close()
		}
		if kafkaCloser != nil {
			kafkaCloser()
		}
		_ = sqlManager.Close()
		_ = redisService.Close()
		HGLoggerPackage.CloseLogger()
		return nil, fmt.Errorf("Video recommend模块初始化失败: %w", err)
	}
	// 视频推荐
	if err := VideoInteractionModulePackage.RegisterModules(redisService, sqlManager); err != nil {
		if coinJobs != nil {
			coinJobs.Close()
		}
		if interactionReprojector != nil {
			interactionReprojector.Close()
		}
		if kafkaCloser != nil {
			kafkaCloser()
		}
		_ = sqlManager.Close()
		_ = redisService.Close()
		HGLoggerPackage.CloseLogger()
		return nil, fmt.Errorf("Video interaction模块初始化失败: %w", err)
	}
	// 注册视频评论模块时传入 Redis/MySQL 依赖，模块内部创建 Handler 时会用到这些依赖构建 Service 和 Handler。
	videoCommentComponents, err := VideoCommentModulePackage.RegisterModules(redisService, sqlManager, CoinTaskPackage.NewHGRedisJobLease(redisService))
	if err != nil {
		if coinJobs != nil {
			coinJobs.Close()
		}
		if interactionReprojector != nil {
			interactionReprojector.Close()
		}
		if kafkaCloser != nil {
			kafkaCloser()
		}
		_ = sqlManager.Close()
		_ = redisService.Close()
		HGLoggerPackage.CloseLogger()
		return nil, fmt.Errorf("Video comment模块初始化失败: %w", err)
	}
	if videoCommentComponents.Maintenance != nil {
		// Maintenance 通常就是评论模块内部的后台维护任务
		videoCommentComponents.Maintenance.Start(context.Background())
	}
	videoDanmakuComponents, err := VideoDanmakuModulePackage.RegisterModules(redisService, sqlManager)
	if err != nil {
		if videoCommentComponents.Maintenance != nil {
			videoCommentComponents.Maintenance.Close()
		}
		if coinJobs != nil {
			coinJobs.Close()
		}
		if interactionReprojector != nil {
			interactionReprojector.Close()
		}
		if kafkaCloser != nil {
			kafkaCloser()
		}
		_ = sqlManager.Close()
		_ = redisService.Close()
		HGLoggerPackage.CloseLogger()
		return nil, fmt.Errorf("Video danmaku模块初始化失败: %w", err)
	}
	// 注册运维管理模块
	opsComponents := OpsModulePackage.RegisterModules(redisService, sqlManager)
	// 读取爬虫任务配置
	crawlerTaskConfig, err := ConfigPackage.GetCrawlerTaskConfig()
	if err != nil {
		return nil, fmt.Errorf("Crawler task配置失败: %w", err)
	}
	// 创建爬虫任务调度器
	crawlerTaskScheduler, err := CrawlerRuntimePackage.NewHGTaskScheduler(
		opsComponents.CrawlerRepo,
		opsComponents.CrawlerTasks,
		crawlerTaskConfig.SchedulerEnabled,
		crawlerTaskConfig.RefreshInterval,
		crawlerTaskConfig.MaxTasks,
	)
	if err != nil {
		return nil, fmt.Errorf("Crawler task scheduler初始化失败: %w", err)
	}
	// 启动爬虫调度器
	if err := crawlerTaskScheduler.Start(context.Background()); err != nil {
		return nil, fmt.Errorf("Crawler task scheduler启动失败: %w", err)
	}
	// Recovery shares the exact repository/service instances used by the API, preserving the same idempotency and audit boundaries.
	correctionRecoveryConfig, err := ConfigPackage.GetCorrectionRecoveryConfig()
	if err != nil {
		if videoCommentComponents.Maintenance != nil {
			videoCommentComponents.Maintenance.Close()
		}
		if coinJobs != nil {
			coinJobs.Close()
		}
		if interactionReprojector != nil {
			interactionReprojector.Close()
		}
		if kafkaCloser != nil {
			kafkaCloser()
		}
		_ = sqlManager.Close()
		_ = redisService.Close()
		HGLoggerPackage.CloseLogger()
		return nil, fmt.Errorf("Correction recovery配置失败: %w", err)
	}
	var correctionRecovery *OpsTaskPackage.HGCorrectionRecovery
	if correctionRecoveryConfig.Enabled {
		correctionRecovery, err = OpsTaskPackage.NewHGCorrectionRecovery(
			opsComponents.Repo,
			opsComponents.Operational,
			OpsTaskPackage.HGCorrectionRecoveryConfig{
				Interval: correctionRecoveryConfig.Interval, Timeout: correctionRecoveryConfig.Timeout,
				ApprovingTimeout: correctionRecoveryConfig.ApprovingTimeout, BatchSize: correctionRecoveryConfig.BatchSize,
			},
			CoinTaskPackage.NewHGRedisJobLease(redisService),
		)
		if err != nil {
			if videoCommentComponents.Maintenance != nil {
				videoCommentComponents.Maintenance.Close()
			}
			if coinJobs != nil {
				coinJobs.Close()
			}
			if interactionReprojector != nil {
				interactionReprojector.Close()
			}
			if kafkaCloser != nil {
				kafkaCloser()
			}
			_ = sqlManager.Close()
			_ = redisService.Close()
			HGLoggerPackage.CloseLogger()
			return nil, fmt.Errorf("Correction recovery初始化失败: %w", err)
		}
		correctionRecovery.Start(context.Background())
	}

	// 主应用模式只托管周期 worker，不直接把 crawler 管理 API 挂到业务端口。
	// 管理 API 仍由独立 cmd/hg_crawler 提供，避免绕过现有 API Gateway 模块策略和鉴权边界。
	crawlerConfig, err := ConfigPackage.GetCrawlerConfig()
	if err != nil {
		if correctionRecovery != nil {
			correctionRecovery.Close()
		}
		if videoCommentComponents.Maintenance != nil {
			videoCommentComponents.Maintenance.Close()
		}
		if coinJobs != nil {
			coinJobs.Close()
		}
		if interactionReprojector != nil {
			interactionReprojector.Close()
		}
		if kafkaCloser != nil {
			kafkaCloser()
		}
		_ = sqlManager.Close()
		_ = redisService.Close()
		HGLoggerPackage.CloseLogger()
		return nil, fmt.Errorf("Crawler配置失败: %w", err)
	}
	var crawlerManager *CrawlerSpiderPackage.HGManager
	if crawlerConfig.Enabled {
		crawlerStore := CrawlerServicePackage.NewHGExternalContentStore(
			CrawlerRepositoryPackage.NewRepository(sqlManager.GetSQLDB()),
			VideoUploadCachePackage.NewCache(redisService),
		)
		crawlerManager, err = CrawlerRuntimePackage.NewHGBilibiliManager(CrawlerRuntimePackage.HGBilibiliRuntimeConfig{
			Interval: crawlerConfig.Interval, Timeout: crawlerConfig.Timeout,
			MaxItems: crawlerConfig.MaxItems, RetryCount: crawlerConfig.RetryCount,
			RatePerSecond: crawlerConfig.RatePerSecond, UserAgent: crawlerConfig.UserAgent,
			Store: crawlerStore,
		})
		if err == nil {
			err = crawlerManager.Start()
		}
		if err != nil {
			if videoDanmakuComponents.Realtime != nil {
				shutdownCtx, cancel := context.WithTimeout(context.Background(), mlcServerShutdownTimeout)
				_ = videoDanmakuComponents.Realtime.Close(shutdownCtx)
				cancel()
			}
			if correctionRecovery != nil {
				correctionRecovery.Close()
			}
			if videoCommentComponents.Maintenance != nil {
				videoCommentComponents.Maintenance.Close()
			}
			if coinJobs != nil {
				coinJobs.Close()
			}
			if interactionReprojector != nil {
				interactionReprojector.Close()
			}
			if kafkaCloser != nil {
				kafkaCloser()
			}
			_ = sqlManager.Close()
			_ = redisService.Close()
			HGLoggerPackage.CloseLogger()
			return nil, fmt.Errorf("Crawler初始化失败: %w", err)
		}
	}
	HGTestHandlerPackage.RegisterModules()

	// 3. 收集所有模块的路由清单
	routeCatalogs := collectRouteCatalogs()

	// 4. 创建根路由。
	// 这里注入 ReadyCheck，让 /readyz 能检查 Redis/MySQL/Kafka，而 /healthz 保持纯进程存活检查。
	// kafkaCloser 非 nil 证明 Kafka 已完成初始化和启动期 Ping；运行期间 ready 检查仍需持续验证 broker 可达性。
	rootMux := HGHandlerPackage.NewBusinessRootHandler(routeCatalogs)
	businessHandler := HGMiddlewarePackage.Chain(
		apiGateway.Middleware(rootMux),
		HGMiddlewarePackage.RequestIDMiddleware,
		HGMiddlewarePackage.AccessLogMiddleware,
		HGMiddlewarePackage.RecoverMiddleware,
		HGMiddlewarePackage.CORSMiddleware,
	)
	// Component writers expose process-local snapshots only; scraping /metrics never performs MySQL, Redis, Kafka, or other external I/O.
	managementMux := HGHandlerPackage.NewManagementHandler(HGHandlerPackage.HealthCheckConfig{
		ReadyCheck:     newReadyCheck(redisService, sqlManager, kafkaCloser != nil, kafkaRuntime, videoDanmakuComponents.Realtime),
		MetricsHandler: HGKafkaPackage.HGKafkaMetricsHandler(apiGateway.HGWritePrometheusMetrics, StatisticConsumerPackage.HGWritePrometheusMetrics, VideoInteractionRepositoryPackage.HGWritePrometheusMetrics, VideoInteractionTaskPackage.HGWritePrometheusMetrics, CoinRepositoryPackage.HGWritePrometheusMetrics, CoinTaskPackage.HGWritePrometheusMetrics, OpsRepositoryPackage.HGWritePrometheusMetrics, VideoCommentTaskPackage.HGWritePrometheusMetrics, videoDanmakuComponents.Realtime.HGWritePrometheusMetrics),
	})

	srv := &http.Server{
		Addr:    buildListenAddr(ConfigPackage.GetServerPort()),
		Handler: businessHandler,
		// ReadHeaderTimeout/ReadTimeout/WriteTimeout/IdleTimeout 是标准库 HTTP 服务的资源治理边界。
		// 没有这些边界时，慢客户端或异常流量会长时间占用连接、goroutine 和内存。
		ReadHeaderTimeout: mlcServerReadHeaderTimeout,
		ReadTimeout:       mlcServerReadTimeout,
		WriteTimeout:      mlcServerWriteTimeout,
		IdleTimeout:       mlcServerIdleTimeout,
		MaxHeaderBytes:    mlcServerMaxHeaderBytes,
	}
	managementServer := &http.Server{
		Addr:              buildManagementListenAddr(ConfigPackage.GetManagementHost(), ConfigPackage.GetManagementPort()),
		Handler:           managementMux,
		ReadHeaderTimeout: mlcServerReadHeaderTimeout,
		ReadTimeout:       mlcManagementReadTimeout,
		WriteTimeout:      mlcManagementWriteTimeout,
		IdleTimeout:       mlcServerIdleTimeout,
		MaxHeaderBytes:    mlcServerMaxHeaderBytes,
	}

	return &MLCApplication{
		server:                  srv,
		managementServer:        managementServer,
		redisService:            redisService,
		sqlManager:              sqlManager,
		kafkaCloser:             kafkaCloser,
		kafkaRuntime:            kafkaRuntime,
		interactionReprojector:  interactionReprojector,
		coinJobs:                coinJobs,
		correctionRecovery:      correctionRecovery,
		videoCommentMaintenance: videoCommentComponents.Maintenance,
		videoDanmakuRealtime:    videoDanmakuComponents.Realtime,
		crawlerManager:          crawlerManager,
		crawlerTaskScheduler:    crawlerTaskScheduler,
	}, nil
}

// newReadyCheck 聚合 Redis/MySQL/Kafka 依赖检查，供 /readyz 区分依赖是否可用。
//
// /healthz 只说明进程还活着；/readyz 说明依赖可用、实例可以接业务流量。
// Kubernetes/负载均衡可以据此在依赖不可用时摘掉实例，避免把请求打到不可服务的节点。
// kafkaEnabled 表达本次启动是否已成功初始化 Kafka；生产启动成功后该值必须为 true。
// Kafka 已初始化时必须持续检查，防止 broker 故障后实例继续被流量入口视为 ready。
func newReadyCheck(redisService *PersistenceRedisPackage.RedisService, sqlManager *PersistenceSQLPackage.HGSQLManager, kafkaEnabled bool, dependencies ...interface{ Ready() error }) HGHandlerPackage.DependencyChecker {
	return func(ctx context.Context) error {
		if err := redisService.PingContext(ctx); err != nil {
			return fmt.Errorf("redis not ready: %w", err)
		}
		if err := sqlManager.PingContext(ctx); err != nil {
			return fmt.Errorf("mysql not ready: %w", err)
		}
		if kafkaEnabled {
			if err := HGKafkaPackage.HGPingKafka(ctx); err != nil {
				return fmt.Errorf("kafka not ready: %w", err)
			}
		}
		for _, dependency := range dependencies {
			if dependency != nil {
				if err := dependency.Ready(); err != nil {
					return fmt.Errorf("runtime dependency not ready: %w", err)
				}
			}
		}
		return nil
	}
}

func hgKafkaRuntimeReadyCheck(runtime kafkaReadyChecker) HGHandlerPackage.DependencyChecker {
	return func(context.Context) error {
		if runtime == nil {
			return nil
		}
		return runtime.Ready()
	}
}

// newRedisServiceWithRecover 统一兜底 Redis 初始化失败，避免入口构建阶段直接退出进程。
// 使用带超时的 context 是为了让启动失败快速返回，而不是无限等待网络连接。
func newRedisServiceWithRecover() (redisService *PersistenceRedisPackage.RedisService, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), infraInitTimeout)
	defer cancel()

	redisService, err = PersistenceRedisPackage.NewRedisServiceWithError(ctx)
	if err != nil {
		return nil, err
	}
	if redisService == nil {
		return nil, errors.New("redis service is nil")
	}
	return redisService, nil
}

// Serve 启动全部监听器，并在收到退出信号时先摘流、再 Drain WebSocket、最后关闭管理面。
//
// 关键设计：
// 1. ListenAndServe 放到 goroutine 中运行，主 goroutine 同时监听系统退出信号。
// 2. 收到 SIGINT/SIGTERM 后调用 Shutdown，而不是 Close，给正在处理的请求一个完成窗口。
// 3. Shutdown 完成后等待 serveErr 返回，确保 HTTP server 的生命周期完整结束。
func (app *MLCApplication) Serve(ctx context.Context) error {
	if app == nil || app.server == nil || app.managementServer == nil {
		return errors.New("mlc application server is nil")
	}

	serverCount := 2
	if app.videoDanmakuRealtime != nil {
		serverCount++
	}
	serveErr := make(chan error, serverCount)
	go func() {
		serveErr <- serveNamedMLCServer("business", app.server)
	}()
	go func() {
		serveErr <- serveNamedMLCServer("management", app.managementServer)
	}()
	if app.videoDanmakuRealtime != nil {
		// 基于 gnet（高性能事件驱动网络库） 的实时弹幕服务启动逻辑；
		// 外层：启动 videoDanmakuRealtime.Server 放到 goroutine，错误投递到 serveErr channel。
		go func() { serveErr <- app.videoDanmakuRealtime.Serve() }()
	}
	/** 监听系统信号（SIGINT Ctrl+C、SIGTERM kill）
	创建一个 ctx，收到SIGINT(Ctrl+C) / SIGTERM(kill) 自动 cancel。
	*/
	stopCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	// 释放信号监听，避免 goroutine 泄漏，标准写法
	defer stop()

	select {
	// 任意一个服务（http/gnet 弹幕服务）异常退出，触发关闭流程；比如：gnet 弹幕服务崩溃、HTTP 服务端口冲突，其中一个Serve()返回错误，写入serveErr。
	case err := <-serveErr:
		// hgShutdown 执行整套资源释放：HTTP 服务关闭、弹幕服务Drain（排空）→Close 关闭
		shutdownErr := app.hgShutdown()
		// hgWaitServeErrors()：等待 N 个服务实例全部退出，带超时，收集各个服务返回的错误；已经收到1 个服务退出 err，还需要等待剩下 serverCount‑1 个服务退出。
		// 使用 Go1.20+ errors.Join 聚合多个错误，把关闭过程中所有错误打包返回给上层
		return errors.Join(err, shutdownErr, hgWaitServeErrors(serveErr, serverCount-1, mlcServerShutdownTimeout))
	case <-stopCtx.Done(): // 收到操作系统终止信号，触发关闭流程
		return errors.Join(app.hgShutdown(), hgWaitServeErrors(serveErr, serverCount, mlcServerShutdownTimeout))
	}
}

// hgShutdown 保持 management 存活到最后，使 Drain 期间的 healthz、readyz 和 metrics 仍可观察。
// 每个阶段使用独立 deadline，前一阶段超时不会跳过后续 listener 的关闭。
func (app *MLCApplication) hgShutdown() error {
	var shutdownErrors []error
	if app.videoDanmakuRealtime != nil {
		app.videoDanmakuRealtime.BeginDrain()
	}
	if err := hgShutdownHTTPServer(app.server, mlcServerShutdownTimeout); err != nil {
		shutdownErrors = append(shutdownErrors, fmt.Errorf("HTTP server shutdown failed: %w", err))
	}
	if app.videoDanmakuRealtime != nil {
		drainCtx, cancelDrain := context.WithTimeout(context.Background(), app.videoDanmakuRealtime.DrainTimeout())
		drainTimedOut := false
		if err := app.videoDanmakuRealtime.WaitForDrain(drainCtx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("danmaku realtime drain failed: %w", err))
		} else if errors.Is(err, context.DeadlineExceeded) {
			drainTimedOut = true
		}
		cancelDrain()

		closeCtx, cancelClose := context.WithTimeout(context.Background(), mlcServerShutdownTimeout)
		if err := app.videoDanmakuRealtime.Close(closeCtx); err != nil {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("danmaku realtime shutdown failed: %w", err))
		}
		cancelClose()
		if drainTimedOut {
			// ServiceMonitor 每 15 秒抓取一次；额外保留 20 秒，让 timeout、force-close 和最终耗时至少跨过一个抓取周期。
			time.Sleep(20 * time.Second)
		}
	}
	if err := hgShutdownHTTPServer(app.managementServer, mlcServerShutdownTimeout); err != nil {
		shutdownErrors = append(shutdownErrors, fmt.Errorf("management server shutdown failed: %w", err))
	}
	return errors.Join(shutdownErrors...)
}

func hgShutdownHTTPServer(server *http.Server, timeout time.Duration) error {
	if server == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return server.Shutdown(ctx)
}

func hgWaitServeErrors(serveErr <-chan error, count int, timeout time.Duration) error {
	var serveErrors []error
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for i := 0; i < count; i++ {
		select {
		case err := <-serveErr:
			if err != nil {
				serveErrors = append(serveErrors, err)
			}
		case <-timer.C:
			serveErrors = append(serveErrors, fmt.Errorf("%d server listeners did not stop within %s", count-i, timeout))
			return errors.Join(serveErrors...)
		}
	}
	return errors.Join(serveErrors...)
}

// Close 释放应用持有的基础设施资源。
// HTTP server 的关闭由 Serve 中的 Shutdown 负责；这里释放 Redis/MySQL/Logger。
// 分层关闭可以避免连接池在请求仍未结束时被提前关闭。
func (app *MLCApplication) Close() {
	if app == nil {
		return
	}
	// 先停止并等待后台 worker，避免数据库和 Redis 关闭后仍有投影或消息处理。
	if app.interactionReprojector != nil {
		app.interactionReprojector.Close()
	}
	if app.coinJobs != nil {
		app.coinJobs.Close()
	}
	if app.correctionRecovery != nil {
		app.correctionRecovery.Close()
	}
	if app.videoCommentMaintenance != nil {
		app.videoCommentMaintenance.Close()
	}
	// crawler 只依赖第三方 HTTP，但必须在 Logger 关闭前停止，确保退出期错误仍可记录。
	if app.crawlerTaskScheduler != nil {
		_ = app.crawlerTaskScheduler.Close()
	}
	if app.crawlerManager != nil {
		app.crawlerManager.Stop()
	}
	// 实时网关依赖 Redis/MySQL，必须在连接池关闭前停止 worker 和全部 WebSocket 连接。
	if app.videoDanmakuRealtime != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), mlcServerShutdownTimeout)
		_ = app.videoDanmakuRealtime.Close(shutdownCtx)
		cancel()
	}
	if app.kafkaCloser != nil {
		app.kafkaCloser()
	}
	if err := app.redisService.Close(); err != nil {
		logHG.ErrFInfo("Redis关闭失败: %v", err)
	}
	if err := app.sqlManager.Close(); err != nil {
		logHG.ErrFInfo("数据库关闭失败: %v", err)
	}
	HGLoggerPackage.CloseLogger()
}

// collectRouteCatalogs 收集所有已注册模块的路由清单，供 App/Web 联调用。
func collectRouteCatalogs() []HGMiddlewareGroupPackage.HGRouteCatalogItem {
	items := make([]HGMiddlewareGroupPackage.HGRouteCatalogItem, 0, 16)

	// 收集 auth 模块路由清单
	items = append(items, HGMiddlewareGroupPackage.AuthRouteCatalog()...)
	// 收集 user 模块路由清单
	items = append(items, HGMiddlewareGroupPackage.UserRouteCatalog()...)
	// 收集 video_upload 模块路由清单
	items = append(items, HGMiddlewareGroupPackage.VideoUploadRouteCatalog()...)
	items = append(items, HGMiddlewareGroupPackage.BilibiliRouteCatalog()...)
	items = append(items, HGMiddlewareGroupPackage.VideoRecommendRouteCatalog()...)
	items = append(items, HGMiddlewareGroupPackage.VideoInteractionRouteCatalog()...)
	items = append(items, HGMiddlewareGroupPackage.VideoCommentRouteCatalog()...)
	items = append(items, HGMiddlewareGroupPackage.VideoDanmakuRouteCatalog()...)
	// 收集 ops 模块路由清单
	items = append(items, HGMiddlewareGroupPackage.OpsRouteCatalog()...)
	// 收集 test 模块路由清单
	items = append(items, HGTestHandlerPackage.TestRouteCatalog()...)

	return items
}

// serveMLCServer 统一处理 ListenAndServe 返回错误，避免直接退出进程导致 defer 失效。
func serveMLCServer(srv *http.Server) error {
	/** ListenAndServe()：开启 HTTP 服务，阻塞调用，直到服务退出。
	返回非 nil err 代表服务终止。
	正常优雅关闭时（调用 srv.Shutdown(ctx)），ListenAndServe 返回的错误固定是：http.ErrServerClosed。
	http.ErrServerClosed：这不是异常错误，是正常关闭的标记错误。
	*/
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

func serveNamedMLCServer(name string, srv *http.Server) error {
	if err := serveMLCServer(srv); err != nil {
		return fmt.Errorf("%s server: %w", name, err)
	}
	return nil
}

// buildListenAddr 统一处理端口配置，兼容 "8080" 和 ":8080" 两种写法。
func buildListenAddr(port string) string {
	if strings.HasPrefix(port, ":") {
		return port
	}

	return ":" + port
}

func buildManagementListenAddr(host string, port string) string {
	// 把 host 和 port 拼接成标准网络地址 host:port 格式，并且先去掉 port 前面的冒号。注意：若是ipv6使用普通字符串拼接会出现问题，需要使用net.JoinHostPort
	return net.JoinHostPort(host, strings.TrimPrefix(port, ":"))
}
