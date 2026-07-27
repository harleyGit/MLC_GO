package statistic

import (
	"MLC_GO/internal/events"
	VideoEventsPackage "MLC_GO/internal/events/video"
	"context"
	"errors"
	"testing"
)

type hgCounterStub struct {
	eventID string
	metric  string
	err     error
}

func (s *hgCounterStub) Increment(_ context.Context, eventID string, metric string) error {
	s.eventID = eventID
	s.metric = metric
	return s.err
}

func TestConsumerIncrementsIdempotentEventCounter(t *testing.T) {
	counter := &hgCounterStub{}
	err := NewConsumer(counter).Handle(context.Background(), events.EventEnvelope{
		EventID:   "event-1",
		EventName: VideoEventsPackage.VideoPublishedEventName,
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if counter.eventID != "event-1" || counter.metric != VideoEventsPackage.VideoPublishedEventName {
		t.Fatalf("counter = %#v, want event id and event name", counter)
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
	err := NewConsumer(&hgCounterStub{err: want}).Handle(context.Background(), events.EventEnvelope{
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
