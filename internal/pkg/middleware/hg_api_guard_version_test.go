package HGMiddlewarePackage

import (
	"net/http"
	"testing"
)

func TestAPIGuardDoesNotFallbackUnknownVersion(t *testing.T) {
	guard := NewAPIGuard([]APIRule{{Path: "/info", Version: "v1", Methods: map[string]bool{http.MethodGet: true}}})
	if _, ok := guard.lookupRule("v2", "/info"); ok {
		t.Fatal("unknown API version unexpectedly fell back to v1")
	}
}
