package eventbus

import (
	"MLC_GO/internal/events"
	VideoEventsPackage "MLC_GO/internal/events/video"
	"context"
	"encoding/json"
	"testing"
)

func TestNewKafkaEnvelopeBuildsConsumerProtocol(t *testing.T) {
	event := VideoEventsPackage.VideoPublishedEvent{
		EventMeta:    events.NewEventMeta(context.Background()),
		SubmissionID: "submission-1",
		UserID:       "user-1",
	}

	envelope, err := newKafkaEnvelope(event)
	if err != nil {
		t.Fatalf("newKafkaEnvelope() error = %v", err)
	}
	if envelope.EventID == "" || envelope.EventName != VideoEventsPackage.VideoPublishedEventName || envelope.EventKey != "submission-1" {
		t.Fatalf("envelope = %#v, want consumer protocol fields", envelope)
	}
	if _, err := json.Marshal(envelope); err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
}
