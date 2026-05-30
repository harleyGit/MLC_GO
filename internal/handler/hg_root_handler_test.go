package HGHandlerPackage

import (
	HGMiddlewareGroupPackage "MLC_GO/internal/pkg/middleware/middleware_group"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestHealthz_ReturnOK(t *testing.T) {
	handler := newHealthzHandler()
	req := httptest.NewRequest(http.MethodGet, healthzPath, nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status=%d, got=%d, body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "ok") {
		t.Fatalf("response should contain ok, body=%s", rec.Body.String())
	}
}

func TestHealthz_MethodGuard(t *testing.T) {
	handler := newHealthzHandler()
	req := httptest.NewRequest(http.MethodPost, healthzPath, nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status=%d, got=%d, body=%s", http.StatusMethodNotAllowed, rec.Code, rec.Body.String())
	}
}

func TestReadyz_ReturnReady(t *testing.T) {
	handler := newReadyzHandler(func(ctx context.Context) error {
		return nil
	})
	req := httptest.NewRequest(http.MethodGet, readyzPath, nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status=%d, got=%d, body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "ready") {
		t.Fatalf("response should contain ready, body=%s", rec.Body.String())
	}
}

func TestReadyz_ReturnServiceUnavailable(t *testing.T) {
	handler := newReadyzHandler(func(ctx context.Context) error {
		return errors.New("dependency down")
	})
	req := httptest.NewRequest(http.MethodGet, readyzPath, nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status=%d, got=%d, body=%s", http.StatusServiceUnavailable, rec.Code, rec.Body.String())
	}
}

func TestReadyz_MethodGuard(t *testing.T) {
	handler := newReadyzHandler(nil)
	req := httptest.NewRequest(http.MethodPost, readyzPath, nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status=%d, got=%d, body=%s", http.StatusMethodNotAllowed, rec.Code, rec.Body.String())
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

func TestRegisterRootPrefixRoutes_RejectDeprecatedAuthPath(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodPost, "/api/v1/user/login", nil)
	rec := httptest.NewRecorder()
	rootMux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status=%d, got=%d", http.StatusNotFound, rec.Code)
	}
}

func TestNewRouteCatalogHandler_MethodGuard(t *testing.T) {
	catalog := make([]HGMiddlewareGroupPackage.HGRouteCatalogItem, 0, 16)
	catalog = append(catalog, HGMiddlewareGroupPackage.AuthRouteCatalog()...)
	catalog = append(catalog, HGMiddlewareGroupPackage.UserRouteCatalog()...)

	handler := newRouteCatalogHandler(catalog)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/routes", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status=%d, got=%d, body=%s", http.StatusMethodNotAllowed, rec.Code, rec.Body.String())
	}
}

func TestNewRouteCatalogHandler_ReturnCatalog(t *testing.T) {
	catalog := make([]HGMiddlewareGroupPackage.HGRouteCatalogItem, 0, 16)
	catalog = append(catalog, HGMiddlewareGroupPackage.AuthRouteCatalog()...)
	catalog = append(catalog, HGMiddlewareGroupPackage.UserRouteCatalog()...)

	handler := newRouteCatalogHandler(catalog)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/routes", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status=%d, got=%d, body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "/api/v1/auth/login") {
		t.Fatalf("response should contain auth login path, body=%s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "/api/v1/profile/list") {
		t.Fatalf("response should contain profile list path, body=%s", rec.Body.String())
	}
}

func TestNewRouteCatalogGroupedHandler_MethodGuard(t *testing.T) {
	catalog := make([]HGMiddlewareGroupPackage.HGRouteCatalogItem, 0, 16)
	catalog = append(catalog, HGMiddlewareGroupPackage.AuthRouteCatalog()...)
	catalog = append(catalog, HGMiddlewareGroupPackage.UserRouteCatalog()...)

	grouped := buildRouteCatalogGrouped(catalog)
	handler := newRouteCatalogGroupedHandler(grouped)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/routes/groups", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status=%d, got=%d, body=%s", http.StatusMethodNotAllowed, rec.Code, rec.Body.String())
	}
}

func TestNewRouteCatalogGroupedHandler_ReturnGroups(t *testing.T) {
	catalog := make([]HGMiddlewareGroupPackage.HGRouteCatalogItem, 0, 16)
	catalog = append(catalog, HGMiddlewareGroupPackage.AuthRouteCatalog()...)
	catalog = append(catalog, HGMiddlewareGroupPackage.UserRouteCatalog()...)

	grouped := buildRouteCatalogGrouped(catalog)
	handler := newRouteCatalogGroupedHandler(grouped)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/routes/groups", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status=%d, got=%d, body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "\"auth\"") {
		t.Fatalf("response should contain auth group, body=%s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "\"profile\"") {
		t.Fatalf("response should contain profile group, body=%s", rec.Body.String())
	}
}
