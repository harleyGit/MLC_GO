package feed

import (
	"MLC_GO/internal/consumer"
	"MLC_GO/internal/events"
	VideoEventsPackage "MLC_GO/internal/events/video"
	"context"
	"errors"
	"testing"
)

func TestConsumerRejectsSupportedEventUntilImplemented(t *testing.T) {
	err := NewConsumer().Handle(context.Background(), events.EventEnvelope{EventName: VideoEventsPackage.VideoPublishedEventName})
	if !errors.Is(err, consumer.ErrHandlerNotImplemented) {
		t.Fatalf("error = %v, want ErrHandlerNotImplemented", err)
	}
}

func TestConsumerIgnoresUnrelatedEvent(t *testing.T) {
	if err := NewConsumer().Handle(context.Background(), events.EventEnvelope{EventName: "user.registered"}); err != nil {
		t.Fatalf("unrelated event error = %v", err)
	}
}
