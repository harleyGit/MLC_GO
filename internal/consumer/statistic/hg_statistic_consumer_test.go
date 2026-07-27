package statistic

import (
	"MLC_GO/internal/events"
	VideoEventsPackage "MLC_GO/internal/events/video"
	"context"
	"errors"
	"testing"
)

type hgCounterStub struct {
	delivery Delivery
	eventID  string
	metric   string
	err      error
}

func (s *hgCounterStub) Increment(_ context.Context, delivery Delivery, eventID string, metric string) error {
	s.delivery = delivery
	s.eventID = eventID
	s.metric = metric
	return s.err
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
