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
