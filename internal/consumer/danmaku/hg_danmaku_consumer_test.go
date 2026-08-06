package danmaku

import (
	"MLC_GO/internal/consumer"
	"MLC_GO/internal/events"
	VideoDanmakuEventsPackage "MLC_GO/internal/events/video_danmaku"
	ClickHousePackage "MLC_GO/internal/pkg/clickhouse"
	"context"
	"encoding/json"
	"testing"
)

type hgHistoryStoreStub struct {
	items []ClickHousePackage.HGDanmakuHistory
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
