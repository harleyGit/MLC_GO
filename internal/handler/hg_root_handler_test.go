package HGHandlerPackage

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegisterRootPrefixRoutes_ForwardWithStripPrefix(t *testing.T) {
	rootMux := http.NewServeMux()
	mockModuleMux := http.NewServeMux()
	mockModuleMux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/login" {
			t.Fatalf("unexpected module path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	registerRootPrefixRoutes(rootMux, []HGRouteMount{
		{
			Prefix:      "/api/v1/auth/",
			StripPrefix: "/api/v1/auth",
			Handler:     mockModuleMux,
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	rec := httptest.NewRecorder()

	rootMux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status=%d, got=%d", http.StatusOK, rec.Code)
	}
	if rec.Body.String() != "ok" {
		t.Fatalf("expected body=%q, got=%q", "ok", rec.Body.String())
	}
}

func TestRegisterRootPrefixRoutes_IgnoreInvalidMount(t *testing.T) {
	rootMux := http.NewServeMux()
	registerRootPrefixRoutes(rootMux, []HGRouteMount{
		{
			Prefix:      "/invalid/",
			StripPrefix: "",
			Handler:     http.NewServeMux(),
		},
		{
			Prefix:      "",
			StripPrefix: "/invalid",
			Handler:     http.NewServeMux(),
		},
		{
			Prefix:      "/invalid/",
			StripPrefix: "/invalid",
			Handler:     nil,
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/invalid/ping", nil)
	rec := httptest.NewRecorder()
	rootMux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status=%d, got=%d", http.StatusNotFound, rec.Code)
	}
}

func TestRegisterRootPrefixRoutes_RejectLegacyPath(t *testing.T) {
	rootMux := http.NewServeMux()
	mockModuleMux := http.NewServeMux()
	mockModuleMux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	registerRootPrefixRoutes(rootMux, []HGRouteMount{
		{
			Prefix:      "/api/v1/auth/",
			StripPrefix: "/api/v1/auth",
			Handler:     mockModuleMux,
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	rec := httptest.NewRecorder()
	rootMux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status=%d, got=%d", http.StatusNotFound, rec.Code)
	}
}
