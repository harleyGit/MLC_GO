package danmaku

import (
	"MLC_GO/internal/consumer"
	"MLC_GO/internal/events"
	VideoDanmakuEventsPackage "MLC_GO/internal/events/video_danmaku"
	ClickHousePackage "MLC_GO/internal/pkg/clickhouse"
	HGKafkaPackage "MLC_GO/internal/pkg/kafka"
	"context"
	"encoding/json"
	"testing"
)

type hgHistoryStoreStub struct {
	items []ClickHousePackage.HGDanmakuHistory
}

func TestConsumerMarksMalformedEventAsTerminalBatchRecord(t *testing.T) {
	handler := NewConsumer(&hgHistoryStoreStub{}, &hgRecentProjectorStub{})
	delivered := []consumer.DeliveredEnvelope{
		{Delivery: consumer.Delivery{Topic: "danmaku", Partition: 3, Offset: 7}, Envelope: events.EventEnvelope{EventName: VideoDanmakuEventsPackage.VideoDanmakuCreatedEventName, Payload: json.RawMessage(`{"danmakuId":"DMK_1","videoId":"video-1","userId":"user-1","createdAt":1000}`)}},
		{Delivery: consumer.Delivery{Topic: "danmaku", Partition: 3, Offset: 8}, Envelope: events.EventEnvelope{EventName: VideoDanmakuEventsPackage.VideoDanmakuCreatedEventName, Payload: json.RawMessage(`{"danmakuId":`)}},
	}

	err := handler.HandleBatch(context.Background(), delivered)
	if err == nil || !HGKafkaPackage.HGIsTerminalError(err) {
		t.Fatalf("HandleBatch() error = %v, want terminal error", err)
	}
	if index := HGKafkaPackage.HGBatchFailureIndex(err, len(delivered)); index != 1 {
		t.Fatalf("batch failure index = %d, want 1", index)
	}
}

func TestConsumerMarksMissingStableFieldsAsTerminal(t *testing.T) {
	handler := NewConsumer(&hgHistoryStoreStub{}, &hgRecentProjectorStub{})
	payload, _ := json.Marshal(VideoDanmakuEventsPackage.CreatedEvent{DanmakuID: "DMK_1", VideoID: "video-1"})
	err := handler.HandleBatch(context.Background(), []consumer.DeliveredEnvelope{{
		Delivery: consumer.Delivery{Topic: "danmaku", Partition: 3, Offset: 7},
		Envelope: events.EventEnvelope{EventName: VideoDanmakuEventsPackage.VideoDanmakuCreatedEventName, Payload: payload},
	}})
	if err == nil || !HGKafkaPackage.HGIsTerminalError(err) {
		t.Fatalf("HandleBatch() error = %v, want terminal error", err)
	}
}

func (s *hgHistoryStoreStub) StoreDanmakuHistory(_ context.Context, items []ClickHousePackage.HGDanmakuHistory) error {
	s.items = append(s.items, items...)
	return nil
}

type hgRecentProjectorStub struct {
	deliveries []consumer.Delivery
	items      []ClickHousePackage.HGDanmakuHistory
}

func (s *hgRecentProjectorStub) Project(_ context.Context, delivery consumer.Delivery, item ClickHousePackage.HGDanmakuHistory) error {
	s.deliveries = append(s.deliveries, delivery)
	s.items = append(s.items, item)
	return nil
}

func TestConsumerStoresPartitionBatchBeforeRecentProjection(t *testing.T) {
	store, recent := &hgHistoryStoreStub{}, &hgRecentProjectorStub{}
	handler := NewConsumer(store, recent)
	delivered := make([]consumer.DeliveredEnvelope, 0, 2)
	for index, danmakuID := range []string{"DMK_1", "DMK_2"} {
		event := VideoDanmakuEventsPackage.CreatedEvent{DanmakuID: danmakuID, VideoID: "video-1", UserID: "user-1", CreatedAt: int64(1000 + index)}
		payload, _ := json.Marshal(event)
		delivered = append(delivered, consumer.DeliveredEnvelope{
			Delivery: consumer.Delivery{Topic: "danmaku", Partition: 3, Offset: int64(index + 7)},
			Envelope: events.EventEnvelope{EventName: VideoDanmakuEventsPackage.VideoDanmakuCreatedEventName, Payload: payload},
		})
	}
	if err := handler.HandleBatch(context.Background(), delivered); err != nil {
		t.Fatalf("HandleBatch() error = %v", err)
	}
	if len(store.items) != 2 || store.items[0].DanmakuID != "DMK_1" || store.items[1].DanmakuID != "DMK_2" {
		t.Fatalf("stored items = %#v", store.items)
	}
	if len(recent.deliveries) != 2 || recent.deliveries[1].Offset != 8 {
		t.Fatalf("recent deliveries = %#v", recent.deliveries)
	}
}
