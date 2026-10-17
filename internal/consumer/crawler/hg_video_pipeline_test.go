package crawler

import (
	"MLC_GO/internal/consumer"
	"MLC_GO/internal/events"
	CrawlerEventPackage "MLC_GO/internal/events/crawler"
	CrawlerDtoPackage "MLC_GO/internal/modules/crawler/dto"
	CrawlerModelPackage "MLC_GO/internal/modules/crawler/model"
	CrawlerServicePackage "MLC_GO/internal/modules/crawler/service"
	HGKafkaPackage "MLC_GO/internal/pkg/kafka"
	"context"
	"errors"
	"testing"
)

type hgStoreStub struct {
	status string                             // 模拟持久阶段。
	events []events.EventEnvelope             // 已可靠发布的阶段事件。
	fail   bool                               // 模拟事务失败。
	detail *CrawlerModelPackage.HGVideoDetail // 最后落库快照。
}

func (s *hgStoreStub) HGCollectionStatus(context.Context, string, string) (string, error) {
	return s.status, nil
}
func (s *hgStoreStub) HGAdvanceVideo(_ context.Context, _ CrawlerEventPackage.HGVideoEvent, expected, next, _ string, event *events.EventEnvelope, d *CrawlerModelPackage.HGVideoDetail) error {
	if s.fail {
		return errors.New("数据库不可用")
	}
	if s.status != expected {
		return nil
	}
	s.status = next
	s.detail = d
	if event != nil {
		s.events = append(s.events, *event)
	}
	return nil
}

type hgHTTPStub struct {
	calls  int
	status int
}                                                                            // 请求计数及模拟HTTP状态。
func (s *hgHTTPStub) ValidateRequest(CrawlerDtoPackage.HGDebugRequest) error { return nil }
func (s *hgHTTPStub) Execute(context.Context, CrawlerDtoPackage.HGDebugRequest) (CrawlerServicePackage.HGHTTPResult, error) {
	s.calls++
	return CrawlerServicePackage.HGHTTPResult{StatusCode: s.status, Body: []byte(`{"code":0,"data":{"bvid":"BV1GJ411x7h7","aid":123,"title":"测试","desc":"简介","duration":12,"pubdate":1700000000,"owner":{"mid":42,"name":"作者"},"stat":{"view":100,"like":0,"coin":0,"favorite":0,"share":0,"reply":6,"danmaku":3},"pages":[{"cid":7,"page":1,"part":"第一集","duration":12}]}}`)}, nil
}

type hgHistoryStub struct {
	rows []HGHistoryRow
	fail bool
} // 已提交快照及故障开关。
func (s *hgHistoryStub) HGStoreCrawlerHistory(_ context.Context, rows []HGHistoryRow) error {
	if s.fail {
		return errors.New("ClickHouse不可用")
	}
	s.rows = append(s.rows, rows...)
	return nil
}

func TestHGPipelineReplayAndHistoryFailure(t *testing.T) {
	ctx := context.Background()
	store := &hgStoreStub{status: "pending"}
	http := &hgHTTPStub{status: 200}
	history := &hgHistoryStub{}
	item := CrawlerEventPackage.HGVideoEvent{BVID: "BV1GJ411x7h7", CollectionID: "f3e1d50a-b910-4f55-a1e6-6bbfc92f6f39"}
	event, _ := CrawlerEventPackage.HGEnvelope(CrawlerEventPackage.HGDiscovered, item)
	fetch := NewHGStageConsumer(CrawlerEventPackage.HGDiscovered, store, http, nil)
	if err := fetch.Handle(ctx, event); err != nil {
		t.Fatal(err)
	}
	if err := fetch.Handle(ctx, event); err != nil {
		t.Fatal(err)
	}
	if http.calls != 1 || len(store.events) != 1 {
		t.Fatal("重复投递产生重复抓取或消息")
	}
	clean := NewHGStageConsumer(CrawlerEventPackage.HGRaw, store, nil, nil)
	if err := clean.Handle(ctx, store.events[0]); err != nil {
		t.Fatal(err)
	}
	persist := NewHGStageConsumer(CrawlerEventPackage.HGCleaned, store, nil, nil)
	store.fail = true
	if err := persist.Handle(ctx, store.events[1]); err == nil {
		t.Fatal("事务失败不得确认")
	}
	if store.status != "cleaned" {
		t.Fatal("失败推进了阶段")
	}
	store.fail = false
	if err := persist.Handle(ctx, store.events[1]); err != nil {
		t.Fatal(err)
	}
	if store.detail == nil || store.detail.Stats.Reply != 6 {
		t.Fatal("详情未落库")
	}
	projection := NewHGStageConsumer(CrawlerEventPackage.HGStored, nil, nil, history)
	history.fail = true
	if err := projection.Handle(ctx, store.events[2]); err == nil {
		t.Fatal("历史失败不得确认")
	}
	history.fail = false
	if err := projection.HandleBatch(ctx, []consumer.DeliveredEnvelope{{Envelope: store.events[2]}}); err != nil {
		t.Fatal(err)
	}
	if len(history.rows) != 1 || history.rows[0].CollectionID != item.CollectionID {
		t.Fatal("历史身份错误")
	}
}

func TestHGFailedCollectionStillRequiresDLQOnReplay(t *testing.T) {
	store := &hgStoreStub{status: "pending"}
	http := &hgHTTPStub{status: 429}
	event, _ := CrawlerEventPackage.HGEnvelope(CrawlerEventPackage.HGDiscovered, CrawlerEventPackage.HGVideoEvent{BVID: "BV1GJ411x7h7", CollectionID: "f3e1d50a-b910-4f55-a1e6-6bbfc92f6f39"})
	c := NewHGStageConsumer(CrawlerEventPackage.HGDiscovered, store, http, nil)
	for i := 0; i < 2; i++ {
		if err := c.Handle(context.Background(), event); !HGKafkaPackage.HGIsTerminalError(err) {
			t.Fatalf("失败重放仍须DLQ确认: %v", err)
		}
	}
	if http.calls != 1 {
		t.Fatal("限制响应不应重复请求")
	}
}
