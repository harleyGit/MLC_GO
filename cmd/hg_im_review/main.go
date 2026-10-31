// hg_im_review 是独立的私有附件审核进程，仅允许 Go 后台任务通过内部网络调用。
package main

import (
	"MLC_GO/internal/modules/im/review"
	Config "MLC_GO/internal/pkg/config"
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	if err := hgRun(); err != nil {
		log.Fatal("IM 审核进程退出: ", err)
	}
}

func hgRun() error {
	if err := Config.InitRuntimeEnv(); err != nil {
		return fmt.Errorf("读取运行环境失败: %w", err)
	}
	probe := os.Getenv("MLC_IM_REVIEW_FFPROBE")
	if probe == "" {
		var err error
		probe, err = exec.LookPath("ffprobe")
		if err != nil {
			return fmt.Errorf("ffprobe 未安装")
		}
	}
	if !strings.HasPrefix(probe, "/") {
		return fmt.Errorf("ffprobe 必须是绝对路径")
	}
	clamd := os.Getenv("MLC_IM_REVIEW_CLAMD_ADDR")
	if clamd == "" {
		clamd = "127.0.0.1:3310"
	}
	cfg := review.HGConfig{
		Token: os.Getenv("MLC_IM_REVIEW_TOKEN"), Bucket: os.Getenv("MLC_IM_S3_BUCKET"),
		ClamdAddr: clamd, FFprobe: probe, TempDir: os.Getenv("MLC_IM_REVIEW_TEMP_DIR"),
		S3Endpoint: os.Getenv("MLC_IM_S3_ENDPOINT"), S3Region: os.Getenv("MLC_IM_S3_REGION"),
		S3Key: os.Getenv("MLC_IM_REVIEW_S3_ACCESS_KEY"), S3Secret: os.Getenv("MLC_IM_REVIEW_S3_SECRET_KEY"),
	}
	service, err := review.HGNewService(cfg)
	if err != nil {
		return err
	}
	defer service.HGClose()
	addr := os.Getenv("MLC_IM_REVIEW_LISTEN")
	if addr == "" {
		addr = "127.0.0.1:8092"
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil || host != "127.0.0.1" && host != "::1" {
		return fmt.Errorf("审核服务只能监听本机 loopback；跨机器由受控 TLS 代理接入")
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /review", service.HGHandler)
	mux.HandleFunc("GET /readyz", service.HGReady)
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 2 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 24 * time.Second, MaxHeaderBytes: 8 << 10}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	select {
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdown)
	}
}
