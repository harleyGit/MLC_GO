package spider

import (
	"context"
	"net/http"
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

type hgRecommendationStore struct {
	items []CrawlerPlatformPackage.HGRecommendation
	err   error
}

func (s *hgRecommendationStore) UpsertRecommendations(_ context.Context, items []CrawlerPlatformPackage.HGRecommendation) error {
	s.items = append([]CrawlerPlatformPackage.HGRecommendation(nil), items...)
	return s.err
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

func TestHGCrawlerRoutes(t *testing.T) {
	want := []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/api/v1/crawler/dashboard"},
		{method: http.MethodGet, path: "/api/v1/crawler/spiders"},
		{method: http.MethodPost, path: "/api/v1/crawler/spiders/bilibili/start"},
		{method: http.MethodPost, path: "/api/v1/crawler/spiders/bilibili/stop"},
		{method: http.MethodGet, path: "/api/v1/crawler/tasks"},
		{method: http.MethodPost, path: "/api/v1/crawler/tasks"},
		{method: http.MethodGet, path: "/api/v1/crawler/recommendations"},
		{method: http.MethodGet, path: "/healthz"},
	}

	routes := hgCrawlerRoutes(&hgHTTPHandler{})
	if len(routes) != len(want) {
		t.Fatalf("hgCrawlerRoutes() length = %d, want %d", len(routes), len(want))
	}
	for i, route := range routes {
		if route.Method != want[i].method || route.FullPath != want[i].path {
			t.Errorf("route[%d] = %s %s, want %s %s", i, route.Method, route.FullPath, want[i].method, want[i].path)
		}
		if route.Handler == nil {
			t.Errorf("route[%d] handler is nil", i)
		}
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

func TestHGManagerPersistsSuccessfulRecommendations(t *testing.T) {
	platform := &hgBlockingPlatform{started: make(chan struct{}), release: make(chan struct{})}
	store := &hgRecommendationStore{}
	manager, err := NewHGManager(platform, time.Minute, time.Second, store)
	if err != nil {
		t.Fatalf("NewHGManager() error = %v", err)
	}
	close(platform.release)
	task, err := manager.RunOnce(context.Background(), HGCreateTaskRequest{})
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if task.Status != "SUCCESS" || len(store.items) != 1 || store.items[0].ContentID != "BV1" {
		t.Fatalf("task = %#v, stored items = %#v", task, store.items)
	}
}

func TestHGManagerMarksPersistenceFailure(t *testing.T) {
	platform := &hgBlockingPlatform{started: make(chan struct{}), release: make(chan struct{})}
	store := &hgRecommendationStore{err: context.DeadlineExceeded}
	manager, err := NewHGManager(platform, time.Minute, time.Second, store)
	if err != nil {
		t.Fatalf("NewHGManager() error = %v", err)
	}
	close(platform.release)
	task, err := manager.RunOnce(context.Background(), HGCreateTaskRequest{})
	if err == nil || task.Status != "FAILED" {
		t.Fatalf("RunOnce() task = %#v, error = %v", task, err)
	}
	if got := manager.Recommendations(); len(got) != 0 {
		t.Fatalf("Recommendations() = %#v, want previous snapshot preserved", got)
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
