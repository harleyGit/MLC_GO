package HGMiddlewarePackage

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestIDMiddlewareReturnsRequestID(t *testing.T) {
	recorder := httptest.NewRecorder()
	RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/routes", nil))
	if recorder.Header().Get("X-Request-ID") == "" {
		t.Fatal("X-Request-ID is empty")
	}
}

func TestAccessLogResponseWriterCapturesStatusAndBytes(t *testing.T) {
	recorder := httptest.NewRecorder()
	w := &hgAccessLogResponseWriter{ResponseWriter: recorder}
	w.WriteHeader(http.StatusCreated)
	if _, err := w.Write([]byte("ok")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if w.status != http.StatusCreated || w.bytes != 2 {
		t.Fatalf("status/bytes = %d/%d", w.status, w.bytes)
	}
}
