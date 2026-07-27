package consumer

import (
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
}
