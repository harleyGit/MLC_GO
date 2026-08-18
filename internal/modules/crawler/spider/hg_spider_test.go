package spider

import (
	"context"
	"sync"
	"testing"
	"time"

	CrawlerPlatformPackage "MLC_GO/internal/modules/crawler/platform"
)

type hgBlockingPlatform struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

type hgPanicPlatform struct{}

type hgStoppingPlatform struct {
	started  chan struct{}
	canceled chan struct{}
	release  chan struct{}
	once     sync.Once
}

func (p *hgPanicPlatform) Name() string { return "bilibili" }

func (p *hgPanicPlatform) FetchRecommendations(context.Context) ([]CrawlerPlatformPackage.HGRecommendation, error) {
	panic("upstream parser panic")
}

func (p *hgStoppingPlatform) Name() string { return "bilibili" }

func (p *hgStoppingPlatform) FetchRecommendations(ctx context.Context) ([]CrawlerPlatformPackage.HGRecommendation, error) {
	p.once.Do(func() { close(p.started) })
	<-ctx.Done()
	close(p.canceled)
	<-p.release
	return nil, ctx.Err()
}

func (p *hgBlockingPlatform) Name() string { return "bilibili" }

func (p *hgBlockingPlatform) FetchRecommendations(ctx context.Context) ([]CrawlerPlatformPackage.HGRecommendation, error) {
	p.once.Do(func() { close(p.started) })
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-p.release:
		return []CrawlerPlatformPackage.HGRecommendation{{Platform: "bilibili", ContentID: "BV1"}}, nil
	}
}

func TestHGManagerPreventsOverlappingTasks(t *testing.T) {
	platform := &hgBlockingPlatform{started: make(chan struct{}), release: make(chan struct{})}
	manager, err := NewHGManager(platform, time.Minute, time.Second)
	if err != nil {
		t.Fatalf("NewHGManager() error = %v", err)
	}
	done := make(chan error, 1)
	go func() {
		_, runErr := manager.RunOnce(context.Background(), HGCreateTaskRequest{})
		done <- runErr
	}()
	<-platform.started
	if _, err := manager.RunOnce(context.Background(), HGCreateTaskRequest{}); err == nil {
		t.Fatal("RunOnce() expected overlap error")
	}
	close(platform.release)
	if err := <-done; err != nil {
		t.Fatalf("first RunOnce() error = %v", err)
	}
	if got := len(manager.Recommendations()); got != 1 {
		t.Fatalf("Recommendations() length = %d", got)
	}
	tasks := manager.Tasks(0, 20, "")
	if tasks["total"].(int) != 1 {
		t.Fatalf("Tasks() = %#v", tasks)
	}
}

func TestHGManagerRejectsUnsupportedTask(t *testing.T) {
	platform := &hgBlockingPlatform{started: make(chan struct{}), release: make(chan struct{})}
	manager, err := NewHGManager(platform, time.Minute, time.Second)
	if err != nil {
		t.Fatalf("NewHGManager() error = %v", err)
	}
	if _, err := manager.RunOnce(context.Background(), HGCreateTaskRequest{Platform: "douyin"}); err == nil {
		t.Fatal("RunOnce() expected unsupported platform error")
	}
}

func TestHGManagerRecoversTaskPanicAndReleasesExecution(t *testing.T) {
	manager, err := NewHGManager(&hgPanicPlatform{}, time.Minute, time.Second)
	if err != nil {
		t.Fatalf("NewHGManager() error = %v", err)
	}
	first, err := manager.RunOnce(context.Background(), HGCreateTaskRequest{})
	if err == nil || first.Status != "FAILED" {
		t.Fatalf("first RunOnce() task = %#v, error = %v", first, err)
	}
	second, err := manager.RunOnce(context.Background(), HGCreateTaskRequest{})
	if err == nil || second.Status != "FAILED" {
		t.Fatalf("second RunOnce() task = %#v, error = %v", second, err)
	}
	if first.ID == second.ID {
		t.Fatalf("task IDs must be unique: first=%d second=%d", first.ID, second.ID)
	}
}

func TestHGManagerRejectsStartWhileStoppingAndAllowsRestart(t *testing.T) {
	platform := &hgStoppingPlatform{started: make(chan struct{}), canceled: make(chan struct{}), release: make(chan struct{})}
	manager, err := NewHGManager(platform, time.Minute, time.Second)
	if err != nil {
		t.Fatalf("NewHGManager() error = %v", err)
	}
	if err := manager.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	<-platform.started

	stopped := make(chan struct{})
	go func() {
		manager.Stop()
		close(stopped)
	}()
	select {
	case <-platform.canceled:
	case <-time.After(time.Second):
		t.Fatal("Stop() did not cancel the worker request")
	}
	// 平台在收到取消后仍受控地停留在收尾阶段，确保这里稳定观察到 STOPPING 而不是依赖调度时序。
	if status := manager.Spiders()[0]["status"]; status != "STOPPING" {
		t.Fatalf("spider status = %v, want STOPPING", status)
	}
	if err := manager.Start(); err == nil {
		t.Fatal("Start() expected conflict while worker is stopping")
	}
	close(platform.release)
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("Stop() did not wait for worker completion")
	}
	if err := manager.Start(); err != nil {
		t.Fatalf("Start() after Stop() error = %v", err)
	}
	manager.Stop()
}
