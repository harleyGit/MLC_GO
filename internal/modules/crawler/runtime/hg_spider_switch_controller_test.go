package runtime

import (
	CrawlerPlatformPackage "MLC_GO/internal/modules/crawler/platform"
	CrawlerSpiderPackage "MLC_GO/internal/modules/crawler/spider"
	"context"
	"errors"
	"testing"
	"time"
)

type hgSwitchReaderStub struct {
	desired bool  // 测试中的数据库期望状态。
	err     error // 模拟数据库读取失败。
}

func (s *hgSwitchReaderStub) HGDesiredRunning(context.Context) (bool, error) {
	return s.desired, s.err
}

type hgSwitchPlatformStub struct{}

func (hgSwitchPlatformStub) Name() string { return "bilibili" }
func (hgSwitchPlatformStub) FetchRecommendations(context.Context) ([]CrawlerPlatformPackage.HGRecommendation, error) {
	return nil, nil
}

func TestHGSpiderSwitchRestoresAndStops(t *testing.T) {
	manager, err := CrawlerSpiderPackage.NewHGManager(hgSwitchPlatformStub{}, time.Minute, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Stop()
	reader := &hgSwitchReaderStub{desired: true}
	if err := hgReconcileSpiderSwitch(context.Background(), reader, manager); err != nil || !manager.HGRunning() {
		t.Fatalf("期望运行时应从数据库恢复: running=%t err=%v", manager.HGRunning(), err)
	}
	reader.desired = false
	if err := hgReconcileSpiderSwitch(context.Background(), reader, manager); err != nil || manager.HGRunning() {
		t.Fatalf("停止应等待 worker 退出: running=%t err=%v", manager.HGRunning(), err)
	}
}

func TestHGSpiderSwitchFailsClosedWithoutDatabase(t *testing.T) {
	manager, err := CrawlerSpiderPackage.NewHGManager(hgSwitchPlatformStub{}, time.Minute, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	reader := &hgSwitchReaderStub{err: errors.New("数据库不可用")}
	if err := HGRunSpiderSwitch(context.Background(), reader, manager); err == nil || manager.HGRunning() {
		t.Fatalf("读取失败时不能自动启动: running=%t err=%v", manager.HGRunning(), err)
	}
}
