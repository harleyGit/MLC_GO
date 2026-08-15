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
