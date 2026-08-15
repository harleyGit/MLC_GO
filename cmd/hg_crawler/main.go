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

	CrawlerPlatformPackage "MLC_GO/internal/modules/crawler/platform"
	CrawlerSpiderPackage "MLC_GO/internal/modules/crawler/spider"
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
	addr := flag.String("addr", ":8080", "crawler admin API listen address")
	interval := flag.Duration("interval", 5*time.Minute, "automatic crawl interval, minimum 10s")
	timeout := flag.Duration("timeout", 10*time.Second, "single crawl timeout")
	once := flag.Bool("once", false, "fetch one recommendation batch and exit")
	flag.Parse()

	platform, err := CrawlerPlatformPackage.NewHGBilibiliPlatform(nil, CrawlerPlatformPackage.HGBilibiliConfig{RequestTimeout: *timeout})
	if err != nil {
		return fmt.Errorf("creating bilibili platform: %w", err)
	}
	manager, err := CrawlerSpiderPackage.NewHGManager(platform, *interval, *timeout)
	if err != nil {
		return fmt.Errorf("creating crawler manager: %w", err)
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
