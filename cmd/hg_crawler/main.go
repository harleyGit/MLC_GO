package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	CrawlerRepositoryPackage "MLC_GO/internal/modules/crawler/repository"
	CrawlerRuntimePackage "MLC_GO/internal/modules/crawler/runtime"
	CrawlerServicePackage "MLC_GO/internal/modules/crawler/service"
	CrawlerSpiderPackage "MLC_GO/internal/modules/crawler/spider"
	VideoUploadCachePackage "MLC_GO/internal/modules/video_upload/cache"
	ConfigPackage "MLC_GO/internal/pkg/config"
	PersistenceSQLPackage "MLC_GO/internal/pkg/mysql"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
)

func main() {
	if err := hgRun(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// hgRun 负责解析命令参数、装配 Bilibili 平台和 crawler manager，并选择单次任务或常驻服务模式。
// --once 适用于本地验证和 Kubernetes CronJob；常驻模式会同时启动周期 worker 与管理 API。
func hgRun() error {
	addr := flag.String("addr", ":8090", "crawler admin API listen address")
	env := flag.String("env", "debug", "runtime environment: debug, pre or prod")
	configDir := flag.String("config-dir", "./config", "configuration root directory")
	interval := flag.Duration("interval", 5*time.Minute, "automatic crawl interval, minimum 10s")
	timeout := flag.Duration("timeout", 10*time.Second, "single crawl timeout")
	once := flag.Bool("once", false, "fetch one recommendation batch and exit")
	flag.Parse()
	if err := os.Setenv("MLC_CONFIG_DIR", *configDir); err != nil {
		return fmt.Errorf("setting crawler config directory: %w", err)
	}
	if err := ConfigPackage.LoadConfig(*env); err != nil {
		return fmt.Errorf("loading crawler config: %w", err)
	}
	sqlManager, err := PersistenceSQLPackage.NewSQLManager()
	if err != nil {
		return fmt.Errorf("creating crawler sql manager: %w", err)
	}
	defer sqlManager.Close()
	redisService, err := PersistenceRedisPackage.NewRedisServiceWithError(context.Background())
	if err != nil {
		return fmt.Errorf("creating crawler redis service: %w", err)
	}
	defer redisService.Close()
	store := CrawlerServicePackage.NewHGExternalContentStore(
		CrawlerRepositoryPackage.NewRepository(sqlManager.GetSQLDB()),
		VideoUploadCachePackage.NewCache(redisService),
	)

	// 独立命令和主应用共用 runtime 工厂，保证限流、重试和协议客户端不会分叉成两套实现。
	manager, err := CrawlerRuntimePackage.NewHGBilibiliManager(CrawlerRuntimePackage.HGBilibiliRuntimeConfig{
		Interval:      *interval,
		Timeout:       *timeout,
		MaxItems:      12,
		RetryCount:    2,
		RatePerSecond: 0.2,
		UserAgent:     "MLC_GO-HGCrawler/1.0",
		Store:         store,
	})
	if err != nil {
		return err
	}
	defer manager.Stop()

	// 单次模式不会启动 HTTP 服务，任务结果直接写 stdout，便于脚本消费和调度平台采集退出码。
	if *once {
		task, runErr := manager.RunOnce(context.Background(), CrawlerSpiderPackage.HGCreateTaskRequest{Platform: "bilibili", Type: "recommendation", Priority: 5})
		result := map[string]interface{}{"task": task, "recommendations": manager.Recommendations()}
		if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
			return fmt.Errorf("encoding crawl result: %w", err)
		}
		return runErr
	}

	if err := manager.Start(); err != nil {
		return fmt.Errorf("starting crawler worker: %w", err)
	}
	server := &http.Server{Addr: *addr, Handler: CrawlerSpiderPackage.NewHGHTTPHandler(manager), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	serverErr := make(chan error, 1)
	// serverErr 必须带一个缓冲，避免主协程先收到退出信号时 ListenAndServe 的返回发送永久阻塞。
	go func() { serverErr <- server.ListenAndServe() }()

	signalCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	select {
	case <-signalCtx.Done():
	case err := <-serverErr:
		if err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("serving crawler API: %w", err)
		}
	}
	// 先停止接收新请求，再由 defer manager.Stop() 取消正在执行的上游抓取并等待 worker 退出。
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutting down crawler API: %w", err)
	}
	return nil
}
