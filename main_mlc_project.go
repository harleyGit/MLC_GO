package main

import (
	StatisticConsumerPackage "MLC_GO/internal/consumer/statistic"
	HGHandlerPackage "MLC_GO/internal/handler"
	CoinRepositoryPackage "MLC_GO/internal/modules/coin/repository"
	CoinTaskPackage "MLC_GO/internal/modules/coin/task"
	OpsModulePackage "MLC_GO/internal/modules/ops/module"
	OpsRepositoryPackage "MLC_GO/internal/modules/ops/repository"
	OpsTaskPackage "MLC_GO/internal/modules/ops/task"
	HGTestHandlerPackage "MLC_GO/internal/modules/test/handler"
	HGUserModulePackage "MLC_GO/internal/modules/user/module"
	VideoCommentModulePackage "MLC_GO/internal/modules/video_comment/module"
	VideoCommentTaskPackage "MLC_GO/internal/modules/video_comment/task"
	VideoInteractionCachePackage "MLC_GO/internal/modules/video_interaction/cache"
	VideoInteractionModulePackage "MLC_GO/internal/modules/video_interaction/module"
	VideoInteractionRepositoryPackage "MLC_GO/internal/modules/video_interaction/repository"
	VideoInteractionTaskPackage "MLC_GO/internal/modules/video_interaction/task"
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

// buildMLCApplication 负责构建工程依赖、注册模块并组装 HTTP Server。
//
// 构建顺序刻意保持为：Logger -> Redis -> MySQL -> Kafka -> 模块注册 -> 根路由 -> Server。
// 这样任何一步失败都可以释放前面已成功初始化的资源，避免启动失败时连接泄漏。
func buildMLCApplication() (*MLCApplication, error) {
	HGLoggerPackage.Init()

	// 1. 初始化基础设施。
	// Redis/MySQL 属于服务 ready 状态的关键依赖；启动期直接校验能尽早暴露配置和网络问题。
	redisService, err := newRedisServiceWithRecover()
	if err != nil {
		HGLoggerPackage.CloseLogger()
		return nil, fmt.Errorf("Redis初始化失败: %w", err)
	}

	sqlManager, err := PersistenceSQLPackage.NewSQLManager()
	if err != nil {
		logHG.ErrFInfo("数据库初始化失败: %v", err)
		_ = redisService.Close()
		HGLoggerPackage.CloseLogger()
		return nil, fmt.Errorf("数据库初始化失败: %w", err)
	}

	// Kafka 是启动必需依赖：配置缺失、静态校验失败或 broker Ping 失败都会中止应用构建。
	// 成功返回的 closer 会保存到 MLCApplication，在退出阶段统一 flush 并关闭全局 Kafka Client。
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
		interactionReprojector.Start(context.Background())
	}
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
		coinJobs.Start(context.Background())
	}

	// 2. 注册所有模块（每个模块内部创建自己的 handler）。
	// ClearModules 用来避免测试或重复构建应用时，全局注册表被 append 出重复模块。
	// 新增模块只需在此处调用 RegisterModules 即可。
	HGHandlerPackage.ClearModules()
	HGUserModulePackage.RegisterModules(redisService, sqlManager, nil)
	// 注册上传视频模块时传入 Redis/MySQL 依赖，模块内部创建 Handler 时会用到这些依赖构建 Service 和 Handler。
	VideoUploadModulePackage.RegisterModules(redisService, sqlManager)
	VideoInteractionModulePackage.RegisterModules(redisService, sqlManager)
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
		videoCommentComponents.Maintenance.Start(context.Background())
	}
	// 注册运维管理模块
	opsComponents := OpsModulePackage.RegisterModules(redisService, sqlManager)
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
	HGTestHandlerPackage.RegisterModules()

	// 3. 收集所有模块的路由清单
	routeCatalogs := collectRouteCatalogs()

	// 4. 创建根路由。
	// 这里注入 ReadyCheck，让 /readyz 能检查 Redis/MySQL/Kafka，而 /healthz 保持纯进程存活检查。
	// kafkaCloser 非 nil 证明 Kafka 已完成初始化和启动期 Ping；运行期间 ready 检查仍需持续验证 broker 可达性。
	rootMux := HGHandlerPackage.NewBusinessRootHandler(routeCatalogs)
	// Component writers expose process-local snapshots only; scraping /metrics never performs MySQL, Redis, Kafka, or other external I/O.
	managementMux := HGHandlerPackage.NewManagementHandler(HGHandlerPackage.HealthCheckConfig{
		ReadyCheck:     newReadyCheck(redisService, sqlManager, kafkaCloser != nil, kafkaRuntime),
		MetricsHandler: HGKafkaPackage.HGKafkaMetricsHandler(StatisticConsumerPackage.HGWritePrometheusMetrics, VideoInteractionRepositoryPackage.HGWritePrometheusMetrics, VideoInteractionTaskPackage.HGWritePrometheusMetrics, CoinRepositoryPackage.HGWritePrometheusMetrics, CoinTaskPackage.HGWritePrometheusMetrics, OpsRepositoryPackage.HGWritePrometheusMetrics),
	})

	srv := &http.Server{
		Addr:    buildListenAddr(ConfigPackage.GetServerPort()),
		Handler: HGMiddlewarePackage.CORSInterceptor(rootMux),
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
	}, nil
}

// newReadyCheck 聚合 Redis/MySQL/Kafka 依赖检查，供 /readyz 区分依赖是否可用。
//
// /healthz 只说明进程还活着；/readyz 说明依赖可用、实例可以接业务流量。
// Kubernetes/负载均衡可以据此在依赖不可用时摘掉实例，避免把请求打到不可服务的节点。
// kafkaEnabled 表达本次启动是否已成功初始化 Kafka；生产启动成功后该值必须为 true。
// Kafka 已初始化时必须持续检查，防止 broker 故障后实例继续被流量入口视为 ready。
func newReadyCheck(redisService *PersistenceRedisPackage.RedisService, sqlManager *PersistenceSQLPackage.HGSQLManager, kafkaEnabled bool, kafkaRuntime ...kafkaReadyChecker) HGHandlerPackage.DependencyChecker {
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
		if len(kafkaRuntime) > 0 && kafkaRuntime[0] != nil {
			if err := kafkaRuntime[0].Ready(); err != nil {
				return fmt.Errorf("kafka consumer runtime not ready: %w", err)
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

// Serve 启动 HTTP 服务并在收到退出信号时执行优雅关闭。
//
// 关键设计：
// 1. ListenAndServe 放到 goroutine 中运行，主 goroutine 同时监听系统退出信号。
// 2. 收到 SIGINT/SIGTERM 后调用 Shutdown，而不是 Close，给正在处理的请求一个完成窗口。
// 3. Shutdown 完成后等待 serveErr 返回，确保 HTTP server 的生命周期完整结束。
func (app *MLCApplication) Serve(ctx context.Context) error {
	if app == nil || app.server == nil || app.managementServer == nil {
		return errors.New("mlc application server is nil")
	}

	serveErr := make(chan error, 2)
	go func() {
		serveErr <- serveNamedMLCServer("business", app.server)
	}()
	go func() {
		serveErr <- serveNamedMLCServer("management", app.managementServer)
	}()

	stopCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-serveErr:
		shutdownCtx, cancel := context.WithTimeout(context.Background(), mlcServerShutdownTimeout)
		defer cancel()
		_ = app.server.Shutdown(shutdownCtx)
		_ = app.managementServer.Shutdown(shutdownCtx)
		return err
	case <-stopCtx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), mlcServerShutdownTimeout)
		defer cancel()
		if err := app.server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("HTTP server shutdown failed: %w", err)
		}
		if err := app.managementServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("management server shutdown failed: %w", err)
		}
		firstErr := <-serveErr
		secondErr := <-serveErr
		if firstErr != nil {
			return firstErr
		}
		return secondErr
	}
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
	items = append(items, HGMiddlewareGroupPackage.VideoInteractionRouteCatalog()...)
	items = append(items, HGMiddlewareGroupPackage.VideoCommentRouteCatalog()...)
	// 收集 ops 模块路由清单
	items = append(items, HGMiddlewareGroupPackage.OpsRouteCatalog()...)
	// 收集 test 模块路由清单
	items = append(items, HGTestHandlerPackage.TestRouteCatalog()...)

	return items
}

// serveMLCServer 统一处理 ListenAndServe 返回错误，避免直接退出进程导致 defer 失效。
func serveMLCServer(srv *http.Server) error {
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
