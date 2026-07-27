package consumer

import (
	"MLC_GO/internal/events"
	"errors"
	"testing"
)

func TestErrHandlerNotImplementedCanBeDetected(t *testing.T) {
	err := NewHandlerNotImplementedError("feed", "video.published")
	if !errors.Is(err, ErrHandlerNotImplemented) {
		t.Fatalf("error = %v, want ErrHandlerNotImplemented", err)
	}
}

func TestDecodeEnvelopeGeneratesStableLegacyEventID(t *testing.T) {
	value := []byte(`{"eventName":"video.published","eventKey":"submission-1","timestamp":1783152000123,"payload":{"submissionId":"submission-1"}}`)

	first, err := DecodeEnvelope(value)
	if err != nil {
		t.Fatalf("DecodeEnvelope() error = %v", err)
	}
	second, err := DecodeEnvelope(value)
	if err != nil {
		t.Fatalf("DecodeEnvelope() second error = %v", err)
	}
	if first.EventID == "" || first.EventID != second.EventID {
		t.Fatalf("legacy event IDs = %q and %q, want stable non-empty fallback", first.EventID, second.EventID)
	}
	if first.Version != events.EventVersionV1 {
		t.Fatalf("legacy version = %d, want %d", first.Version, events.EventVersionV1)
	}
}

func TestDecodeEnvelopeRejectsUnsupportedVersion(t *testing.T) {
	value := []byte(`{"eventId":"event-1","eventName":"video.published","eventKey":"submission-1","version":2,"timestamp":1783152000123,"payload":{"submissionId":"submission-1"}}`)

	_, err := DecodeEnvelope(value)
	if !errors.Is(err, ErrUnsupportedEnvelopeVersion) {
		t.Fatalf("DecodeEnvelope() error = %v, want ErrUnsupportedEnvelopeVersion", err)
	}
}
