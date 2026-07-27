package feed

import (
	"MLC_GO/internal/events"
	VideoEventsPackage "MLC_GO/internal/events/video"
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type hgFeedProjectorStub struct {
	publishedEventID string
	deletedEventID   string
	submissionID     string
	score            int64
	delivery         Delivery
	err              error
}

func (s *hgFeedProjectorStub) Publish(_ context.Context, delivery Delivery, eventID string, submissionID string, score int64) error {
	s.delivery = delivery
	s.publishedEventID = eventID
	s.submissionID = submissionID
	s.score = score
	return s.err
}

func (s *hgFeedProjectorStub) Delete(_ context.Context, delivery Delivery, eventID string, submissionID string) error {
	s.delivery = delivery
	s.deletedEventID = eventID
	s.submissionID = submissionID
	return s.err
}

func TestConsumerPublishesVideoToFeed(t *testing.T) {
	projector := &hgFeedProjectorStub{}
	envelope := hgFeedEnvelope(t, VideoEventsPackage.VideoPublishedEventName)
	ctx := WithDelivery(context.Background(), Delivery{Topic: "mlc.domain.events", Partition: 7, Offset: 19})

	if err := NewConsumer(projector).Handle(ctx, envelope); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if projector.publishedEventID != "event-1" || projector.submissionID != "submission-1" || projector.score != 1783152000123 {
		t.Fatalf("projector = %#v, want published event fields", projector)
	}
	if projector.delivery.Topic != "mlc.domain.events" || projector.delivery.Partition != 7 || projector.delivery.Offset != 19 {
		t.Fatalf("delivery = %#v, want kafka source coordinates", projector.delivery)
	}
}

func TestConsumerRejectsFeedEventWithoutDeliveryMetadata(t *testing.T) {
	err := NewConsumer(&hgFeedProjectorStub{}).Handle(context.Background(), hgFeedEnvelope(t, VideoEventsPackage.VideoPublishedEventName))
	if err == nil {
		t.Fatal("Handle() error = nil, want missing delivery metadata error")
	}
}

func TestConsumerDeletesVideoFromFeed(t *testing.T) {
	projector := &hgFeedProjectorStub{}
	envelope := hgFeedEnvelope(t, VideoEventsPackage.VideoDeletedEventName)
	ctx := WithDelivery(context.Background(), Delivery{Topic: "mlc.domain.events", Partition: 7, Offset: 20})

	if err := NewConsumer(projector).Handle(ctx, envelope); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if projector.deletedEventID != "event-1" || projector.submissionID != "submission-1" {
		t.Fatalf("projector = %#v, want deleted event fields", projector)
	}
}

func TestConsumerRejectsEventWithoutEventID(t *testing.T) {
	envelope := hgFeedEnvelope(t, VideoEventsPackage.VideoPublishedEventName)
	envelope.EventID = ""

	err := NewConsumer(&hgFeedProjectorStub{}).Handle(context.Background(), envelope)
	if err == nil {
		t.Fatal("Handle() error = nil, want missing event id error")
	}
}

func TestConsumerReturnsProjectorError(t *testing.T) {
	want := errors.New("redis unavailable")
	ctx := WithDelivery(context.Background(), Delivery{Topic: "mlc.domain.events", Partition: 7, Offset: 19})
	err := NewConsumer(&hgFeedProjectorStub{err: want}).Handle(ctx, hgFeedEnvelope(t, VideoEventsPackage.VideoPublishedEventName))
	if !errors.Is(err, want) {
		t.Fatalf("Handle() error = %v, want %v", err, want)
	}
}

func TestConsumerIgnoresReviewedAndUnrelatedEvents(t *testing.T) {
	consumer := NewConsumer(&hgFeedProjectorStub{})
	for _, eventName := range []string{VideoEventsPackage.VideoReviewedEventName, "user.registered"} {
		if err := consumer.Handle(context.Background(), events.EventEnvelope{EventName: eventName}); err != nil {
			t.Fatalf("event %q error = %v", eventName, err)
		}
	}
}

func hgFeedEnvelope(t *testing.T, eventName string) events.EventEnvelope {
	t.Helper()
	payload, err := json.Marshal(VideoEventsPackage.VideoPublishedEvent{
		EventMeta:    events.EventMeta{Timestamp: 1783152000123},
		SubmissionID: "submission-1",
		UserID:       "user-1",
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return events.EventEnvelope{
		EventID:   "event-1",
		EventName: eventName,
		EventKey:  "submission-1",
		Timestamp: 1783152000123,
		Payload:   payload,
	}
}
