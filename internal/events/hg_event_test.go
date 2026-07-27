package events_test

import (
	"MLC_GO/internal/events"
	VideoEventsPackage "MLC_GO/internal/events/video"
	"encoding/json"
	"testing"
)

func TestNewEnvelopeKeepsStableByteContract(t *testing.T) {
	event := VideoEventsPackage.VideoReviewedEvent{
		EventMeta: events.EventMeta{
			Version:       events.EventVersionV1,
			TraceID:       "trace-1",
			RequestID:     "request-1",
			Timestamp:     1783152000123,
			SourceService: events.SourceServiceMLC,
		},
		SubmissionID: "submission_1",
		UserID:       "user_1",
	}

	envelope, err := events.NewEnvelope(event)
	if err != nil {
		t.Fatalf("NewEnvelope() error = %v", err)
	}

	if envelope.EventName != VideoEventsPackage.VideoReviewedEventName {
		t.Fatalf("EventName = %q, want %q", envelope.EventName, VideoEventsPackage.VideoReviewedEventName)
	}
	if envelope.EventKey != "submission_1" {
		t.Fatalf("EventKey = %q, want submission_1", envelope.EventKey)
	}
	if envelope.EventID == "" {
		t.Fatal("EventID is empty, want stable event id for consumer idempotency")
	}
	if envelope.Version != events.EventVersionV1 || envelope.TraceID != "trace-1" || envelope.RequestID != "request-1" || envelope.SourceService != events.SourceServiceMLC {
		t.Fatalf("envelope meta = %#v, want event meta copied", envelope)
	}

	var payload map[string]any
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload["submissionId"] != "submission_1" || payload["userId"] != "user_1" {
		t.Fatalf("payload = %#v, want business fields", payload)
	}
}
