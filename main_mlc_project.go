package main

import (
	ConfigPackage "MLC_GO/internal/config"
	HGHandlerPackage "MLC_GO/internal/handler"
	PersistenceSQLPackage "MLC_GO/internal/infrastructure/persistence/mysql"
	PersistenceRedisPackage "MLC_GO/internal/infrastructure/persistence/redis"
	HGMiddlewarePackage "MLC_GO/internal/interfaces/middleware"
	HGMiddlewareGroupPackage "MLC_GO/internal/interfaces/middleware/middleware_group"
	HGLoggerPackage "MLC_GO/internal/logger"
	HGTestHandlerPackage "MLC_GO/internal/modules/test/handler"
	HGUserModulePackage "MLC_GO/internal/modules/user/module"
	"MLC_GO/internal/pkg/logHG"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func init() {
	// 启动阶段先加载 SQL 相关环境变量，保证后续数据库依赖可直接读取到配置。
	PersistenceSQLPackage.LoadSQLEnvValue()
}

// mlc_main 是 MLC_GO 工程入口，负责配置加载、依赖构建与 HTTP 服务启动。
func mlc_main() {
	logHG.DebugInfo("MLC_GO项目启动中...")

	if err := loadRuntimeConfig(); err != nil {
		logHG.ErrFInfo("MLC_GO工程启动失败: %v", err)
		return
	}

	HGLoggerPackage.Init()
	defer HGLoggerPackage.CloseLogger()

	srv, err := buildMLCServer()
	if err != nil {
		logHG.ErrFInfo("MLC_GO工程启动失败: %v", err)
		return
	}

	logHG.DebugFInfo("HTTP server 开始监听: %s", srv.Addr)
	if err := serveMLCServer(srv); err != nil {
		logHG.ErrFInfo("HTTP server 运行失败: %v", err)
	}
}

// loadRuntimeConfig 负责按当前 SERVER_ENV 加载对应的业务配置文件。
func loadRuntimeConfig() error {
	env := ConfigPackage.GetEnv()
	if err := ConfigPackage.LoadConfig(string(env)); err != nil {
		return fmt.Errorf("加载配置文件失败, env=%s: %w", env, err)
	}

	logHG.DebugFInfo("当前环境: %s", env)
	return nil
}

// buildMLCServer 负责构建工程运行所需依赖，并组装 HTTP Server。
func buildMLCServer() (*http.Server, error) {
	// 1. 初始化基础设施
	redisService := PersistenceRedisPackage.NewRedisService()
	sqlManager, err := PersistenceSQLPackage.NewSQLManager()
	if err != nil {
		logHG.ErrFInfo("数据库初始化失败: %v", err)
		return nil, fmt.Errorf("数据库初始化失败: %w", err)
	}

	// 2. 注册所有模块（每个模块内部创建自己的 handler）
	// 新增模块只需在此处调用 RegisterModules 即可
	HGUserModulePackage.RegisterModules(redisService, sqlManager, nil)
	HGTestHandlerPackage.RegisterModules()

	// 3. 收集所有模块的路由清单
	routeCatalogs := collectRouteCatalogs()

	// 4. 创建根路由
	rootMux := HGHandlerPackage.NewRootHandler(routeCatalogs)

	return &http.Server{
		Addr:         buildListenAddr(ConfigPackage.GetServerPort()),
		Handler:      HGMiddlewarePackage.CORSInterceptor(rootMux),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}, nil
}

// collectRouteCatalogs 收集所有已注册模块的路由清单，供 App/Web 联调用。
func collectRouteCatalogs() []HGMiddlewareGroupPackage.HGRouteCatalogItem {
	items := make([]HGMiddlewareGroupPackage.HGRouteCatalogItem, 0, 16)

	// 收集 auth 模块路由清单
	items = append(items, HGMiddlewareGroupPackage.AuthRouteCatalog()...)
	// 收集 user 模块路由清单
	items = append(items, HGMiddlewareGroupPackage.UserRouteCatalog()...)
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

// buildListenAddr 统一处理端口配置，兼容 "8080" 和 ":8080" 两种写法。
func buildListenAddr(port string) string {
	if strings.HasPrefix(port, ":") {
		return port
	}

	return ":" + port
}
