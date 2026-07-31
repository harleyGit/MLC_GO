package HGMiddlewarePackage

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCORSMiddlewareAllowsConfiguredOriginAndSignedHeaders(t *testing.T) {
	t.Setenv("HG_CORS_ALLOWED_ORIGINS", "http://localhost:5174, https://ops.example.com")

	called := false
	handler := CORSMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/ops/bilibili/tags", nil)
	req.Header.Set("Origin", "https://ops.example.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	req.Header.Set("Access-Control-Request-Headers", "content-type, authorization, x-device-id, x-client-type, x-client-version, x-api-version, x-language, x-request-id, x-timestamp, x-signature")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if called {
		t.Fatal("next handler was called for preflight")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://ops.example.com" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want configured request origin", got)
	}
	if !headerContainsToken(rec.Header().Values("Vary"), "Origin") {
		t.Fatalf("Vary = %q, want Origin", rec.Header().Values("Vary"))
	}
	for _, header := range []string{"X-Device-ID", "X-Client-Type", "X-Client-Version", "X-API-Version", "X-Language", "X-Request-ID", "X-Timestamp", "X-Signature"} {
		if !headerContainsToken(rec.Header().Values("Access-Control-Allow-Headers"), header) {
			t.Fatalf("Access-Control-Allow-Headers = %q, missing %s", rec.Header().Values("Access-Control-Allow-Headers"), header)
		}
	}
}

func TestCORSMiddlewareAllowsLocal5174ByDefault(t *testing.T) {
	t.Setenv("HG_CORS_ALLOWED_ORIGINS", "")
	handler := CORSMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ops/bilibili/tags/list", nil)
	req.Header.Set("Origin", "http://localhost:5174")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5174" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want local 5174 origin", got)
	}
}

func TestCORSMiddlewareRejectsDisallowedPreflight(t *testing.T) {
	t.Setenv("HG_CORS_ALLOWED_ORIGINS", "http://localhost:5174,https://ops.example.com")

	called := false
	handler := CORSMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/ops/bilibili/tags", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if called {
		t.Fatal("next handler was called for disallowed preflight")
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want empty", got)
	}
}

func headerContainsToken(values []string, want string) bool {
	for _, value := range values {
		for _, token := range strings.Split(value, ",") {
			if strings.EqualFold(strings.TrimSpace(token), want) {
				return true
			}
		}
	}
	return false
}
