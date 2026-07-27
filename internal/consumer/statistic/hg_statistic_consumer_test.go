package statistic

import (
	"MLC_GO/internal/events"
	VideoEventsPackage "MLC_GO/internal/events/video"
	ClickHousePackage "MLC_GO/internal/pkg/clickhouse"
	"context"
	"errors"
	"testing"
)

type hgEventStoreStub struct {
	event ClickHousePackage.HGStatisticEvent
	err   error
	calls *[]string
}

func (s *hgEventStoreStub) StoreStatisticEvent(_ context.Context, event ClickHousePackage.HGStatisticEvent) error {
	if s.calls != nil {
		*s.calls = append(*s.calls, "clickhouse")
	}
	s.event = event
	return s.err
}

type hgCounterStub struct {
	delivery Delivery
	eventID  string
	metric   string
	err      error
	calls    *[]string
}

func (s *hgCounterStub) Increment(_ context.Context, delivery Delivery, eventID string, metric string) error {
	if s.calls != nil {
		*s.calls = append(*s.calls, "redis")
	}
	s.delivery = delivery
	s.eventID = eventID
	s.metric = metric
	return s.err
}

func TestConsumerStoresAuthorityBeforeRedisProjection(t *testing.T) {
	calls := make([]string, 0, 2)
	store := &hgEventStoreStub{calls: &calls}
	counter := &hgCounterStub{calls: &calls}
	ctx := WithDelivery(context.Background(), Delivery{Topic: "mlc.domain.events", Partition: 3, Offset: 11})
	err := NewConsumer(counter, store, HGProjectionConfig{RedisGeneration: "v2", RedisShardCount: 64}).Handle(ctx, events.EventEnvelope{
		EventID: "event-1", EventName: VideoEventsPackage.VideoPublishedEventName, EventKey: "submission-1",
		Version: 1, Timestamp: 1720000000123, SourceService: "mlc-go",
		Payload: []byte(`{"submissionId":"submission-1","userId":"user-1"}`),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(calls) != 2 || calls[0] != "clickhouse" || calls[1] != "redis" {
		t.Fatalf("calls = %v, want ClickHouse then Redis", calls)
	}
	if store.event.SubmissionID != "submission-1" || store.event.UserID != "user-1" || store.event.RedisShard != 3 || store.event.KafkaOffset != 11 {
		t.Fatalf("stored event = %#v", store.event)
	}
}

func TestConsumerDoesNotAdvanceRedisWhenAuthorityWriteFails(t *testing.T) {
	want := errors.New("clickhouse unavailable")
	calls := make([]string, 0, 1)
	ctx := WithDelivery(context.Background(), Delivery{Topic: "mlc.domain.events", Partition: 0, Offset: 1})
	err := NewConsumer(&hgCounterStub{calls: &calls}, &hgEventStoreStub{err: want, calls: &calls}, HGProjectionConfig{RedisGeneration: "v2", RedisShardCount: 64}).Handle(ctx, events.EventEnvelope{
		EventID: "event-1", EventName: VideoEventsPackage.VideoReviewedEventName,
		Payload: []byte(`{"submissionId":"submission-1","userId":"user-1"}`),
	})
	if !errors.Is(err, want) {
		t.Fatalf("Handle() error = %v, want %v", err, want)
	}
	if len(calls) != 1 || calls[0] != "clickhouse" {
		t.Fatalf("calls = %v, want only ClickHouse", calls)
	}
}

func TestConsumerIncrementsIdempotentEventCounter(t *testing.T) {
	counter := &hgCounterStub{}
	ctx := WithDelivery(context.Background(), Delivery{Topic: "mlc.domain.events", Partition: 3, Offset: 11})
	err := NewConsumer(counter).Handle(ctx, events.EventEnvelope{
		EventID:   "event-1",
		EventName: VideoEventsPackage.VideoPublishedEventName,
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if counter.eventID != "event-1" || counter.metric != VideoEventsPackage.VideoPublishedEventName {
		t.Fatalf("counter = %#v, want event id and event name", counter)
	}
	if counter.delivery.Partition != 3 || counter.delivery.Offset != 11 {
		t.Fatalf("delivery = %#v, want source offset", counter.delivery)
	}
}

func TestConsumerRejectsSupportedEventWithoutDeliveryMetadata(t *testing.T) {
	err := NewConsumer(&hgCounterStub{}).Handle(context.Background(), events.EventEnvelope{EventID: "event-1", EventName: VideoEventsPackage.VideoPublishedEventName})
	if err == nil {
		t.Fatal("Handle() error = nil, want missing delivery metadata error")
	}
}

func TestConsumerRejectsSupportedEventWithoutEventID(t *testing.T) {
	err := NewConsumer(&hgCounterStub{}).Handle(context.Background(), events.EventEnvelope{EventName: VideoEventsPackage.VideoReviewedEventName})
	if err == nil {
		t.Fatal("Handle() error = nil, want missing event id error")
	}
}

func TestConsumerReturnsCounterError(t *testing.T) {
	want := errors.New("redis unavailable")
	ctx := WithDelivery(context.Background(), Delivery{Topic: "mlc.domain.events", Partition: 3, Offset: 12})
	err := NewConsumer(&hgCounterStub{err: want}).Handle(ctx, events.EventEnvelope{
		EventID:   "event-1",
		EventName: VideoEventsPackage.VideoDeletedEventName,
	})
	if !errors.Is(err, want) {
		t.Fatalf("Handle() error = %v, want %v", err, want)
	}
}

func TestConsumerIgnoresUnrelatedEvent(t *testing.T) {
	if err := NewConsumer(&hgCounterStub{}).Handle(context.Background(), events.EventEnvelope{EventName: "user.registered"}); err != nil {
		t.Fatalf("unrelated event error = %v", err)
	}
}
