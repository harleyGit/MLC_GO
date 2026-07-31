package interaction

import (
	"MLC_GO/internal/consumer"
	"MLC_GO/internal/events"
	"context"
	"encoding/json"
	"testing"
)

type hgFakeStore struct {
	calls int
	event PersistedEvent
}

func (f *hgFakeStore) ApplyEvent(_ context.Context, event PersistedEvent) error {
	f.calls++
	f.event = event
	return nil
}

func (f *hgFakeStore) ApplyEvents(_ context.Context, events []PersistedEvent) error {
	f.calls++
	if len(events) > 0 {
		f.event = events[len(events)-1]
	}
	return nil
}

func TestConsumerMapsInteractionEnvelopeToStore(t *testing.T) {
	store := &hgFakeStore{}
	handler := NewConsumer(store)
	payload, err := json.Marshal(map[string]any{
		"actorUserId":  "user-1",
		"submissionId": "submission-1",
		"action":       "like",
		"active":       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := consumer.WithDelivery(context.Background(), consumer.Delivery{Topic: "mlc.domain.events", Partition: 1, Offset: 9})
	err = handler.Handle(ctx, events.EventEnvelope{
		EventID:   "event-1",
		EventName: "video.interaction.changed",
		EventKey:  "submission-1:user-1:like",
		Version:   1,
		Payload:   payload,
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if store.calls != 1 || store.event.EventID != "event-1" || store.event.KafkaOffset != 9 {
		t.Fatalf("stored event = %#v calls=%d", store.event, store.calls)
	}
}

func TestConsumerPersistsDeliveredEnvelopesAsOneBatch(t *testing.T) {
	store := &hgFakeStore{}
	handler := NewConsumer(store)
	payload := json.RawMessage(`{"actorUserId":"user-1","submissionId":"submission-1","action":"like","active":true}`)
	delivered := []consumer.DeliveredEnvelope{
		{Delivery: consumer.Delivery{Topic: "mlc.domain.events", Partition: 1, Offset: 9}, Envelope: events.EventEnvelope{EventID: "event-1", EventName: "video.interaction.changed", EventKey: "key-1", Payload: payload}},
		{Delivery: consumer.Delivery{Topic: "mlc.domain.events", Partition: 1, Offset: 10}, Envelope: events.EventEnvelope{EventID: "event-2", EventName: "video.interaction.changed", EventKey: "key-2", Payload: payload}},
	}
	if err := handler.HandleBatch(context.Background(), delivered); err != nil {
		t.Fatalf("HandleBatch() error = %v", err)
	}
	if store.calls != 1 || store.event.KafkaOffset != 10 {
		t.Fatalf("calls=%d event=%#v", store.calls, store.event)
	}
}

func TestConsumerIgnoresUnrelatedEvents(t *testing.T) {
	store := &hgFakeStore{}
	handler := NewConsumer(store)

	if err := handler.Handle(context.Background(), events.EventEnvelope{EventName: "video.published"}); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if store.calls != 0 {
		t.Fatalf("store calls = %d, want 0", store.calls)
	}
}
