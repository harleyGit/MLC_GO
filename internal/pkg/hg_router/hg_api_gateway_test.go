package HGRouterPackage

import (
	ConfigPackage "MLC_GO/internal/pkg/config"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

type hgGatewayFakeEval struct {
	allowed int64
	err     error
	key     string
}

func TestHGAPIGatewayMetricsUseFixedModuleLabels(t *testing.T) {
	gateway := hgTestGateway(t, &hgGatewayFakeEval{allowed: 1})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/bilibili/author/homepage", nil)
	gateway.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(httptest.NewRecorder(), request)
	var output strings.Builder
	gateway.HGWritePrometheusMetrics(&output)
	metrics := output.String()
	if !strings.Contains(metrics, `mlc_api_gateway_requests_total{module="bilibili"} 1`) || strings.Contains(metrics, request.RemoteAddr) {
		t.Fatalf("unexpected metrics: %s", metrics)
	}
}

func (f *hgGatewayFakeEval) EvalInt64(_ context.Context, _ string, keys []string, _ ...any) (int64, error) {
	if len(keys) > 0 {
		f.key = keys[0]
	}
	return f.allowed, f.err
}

func hgTestGateway(t *testing.T, eval hgGatewayRateEval, trusted ...netip.Prefix) *HGAPIGateway {
	t.Helper()
	modules := make(map[string]ConfigPackage.HGAPIGatewayModulePolicy, len(hgGatewayModulePaths))
	for _, module := range hgGatewayModulePaths {
		modules[module.name] = ConfigPackage.HGAPIGatewayModulePolicy{Capacity: 10, RefillPerSecond: 2, MaxBodyBytes: 1024, MaxInFlight: 10}
	}
	gateway, err := newHGAPIGateway(eval, ConfigPackage.HGAPIGatewayConfig{
		Enabled: true, MaxURLBytes: 8192, SupportedVersions: map[string]struct{}{"v1": {}},
		TrustedProxyCIDRs: trusted, Modules: modules,
	})
	if err != nil {
		t.Fatalf("newHGAPIGateway() error = %v", err)
	}
	return gateway
}

func TestHGAPIGatewayRejectsVersionMismatchAndOversizedRequest(t *testing.T) {
	gateway := hgTestGateway(t, &hgGatewayFakeEval{allowed: 1})
	tests := []struct {
		name       string
		request    *http.Request
		wantStatus int
	}{
		{
			name: "version mismatch",
			request: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/api/v1/profile/info", nil)
				req.Header.Set("X-API-Version", "v2")
				return req
			}(),
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "body too large",
			request:    httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(strings.Repeat("a", 1025))),
			wantStatus: http.StatusRequestEntityTooLarge,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			gateway.Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Fatal("rejected request reached downstream")
			})).ServeHTTP(recorder, test.request)
			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, test.wantStatus)
			}
		})
	}
}

func TestHGAPIGatewayRejectsInFlightOverflow(t *testing.T) {
	gateway := hgTestGateway(t, &hgGatewayFakeEval{allowed: 1})
	gateway.modules[0].inFlight = make(chan struct{}, 1)
	gateway.modules[0].inFlight <- struct{}{}
	recorder := httptest.NewRecorder()
	gateway.Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("overloaded request reached downstream")
	})).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil))
	if recorder.Code != http.StatusServiceUnavailable || recorder.Header().Get("Retry-After") != "1" {
		t.Fatalf("status/retry-after = %d/%q", recorder.Code, recorder.Header().Get("Retry-After"))
	}
}

func TestHGAPIGatewayReverseProxyPreservesPublicRequest(t *testing.T) {
	var gotPath, gotQuery, gotAuth, gotForwarded string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
		gotAuth, gotForwarded = r.Header.Get("Authorization"), r.Header.Get("X-Forwarded-For")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	modules := make(map[string]ConfigPackage.HGAPIGatewayModulePolicy, len(hgGatewayModulePaths))
	for _, module := range hgGatewayModulePaths {
		modules[module.name] = ConfigPackage.HGAPIGatewayModulePolicy{Capacity: 10, RefillPerSecond: 2, MaxBodyBytes: 1024, MaxInFlight: 10}
	}
	policy := modules["bilibili"]
	policy.UpstreamURL = upstream.URL
	modules["bilibili"] = policy
	gateway, err := newHGAPIGateway(&hgGatewayFakeEval{allowed: 1}, ConfigPackage.HGAPIGatewayConfig{
		Enabled: true, MaxURLBytes: 8192, SupportedVersions: map[string]struct{}{"v1": {}}, Modules: modules,
	})
	if err != nil {
		t.Fatalf("newHGAPIGateway() error = %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bilibili/author/homepage?userId=10001", nil)
	req.RemoteAddr = "203.0.113.9:3456"
	req.Header.Set("Authorization", "Bearer token")
	recorder := httptest.NewRecorder()
	gateway.Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("proxied request reached local handler")
	})).ServeHTTP(recorder, req)
	if recorder.Code != http.StatusNoContent || gotPath != "/api/v1/bilibili/author/homepage" || gotQuery != "userId=10001" || gotAuth != "Bearer token" || gotForwarded != "203.0.113.9" {
		t.Fatalf("status/path/query/auth/forwarded = %d/%q/%q/%q/%q", recorder.Code, gotPath, gotQuery, gotAuth, gotForwarded)
	}
}

func TestHGAPIGatewayPreservesBilibiliHomepageRequest(t *testing.T) {
	eval := &hgGatewayFakeEval{allowed: 1}
	gateway := hgTestGateway(t, eval)
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.URL.Path != "/api/v1/bilibili/author/homepage" || r.URL.Query().Get("userId") != "10001" || r.URL.Query().Get("pageSize") != "20" {
			t.Fatalf("gateway changed request URL: %s", r.URL.String())
		}
		w.WriteHeader(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bilibili/author/homepage?userId=10001&pageSize=20", nil)
	req.RemoteAddr = "203.0.113.9:3456"
	recorder := httptest.NewRecorder()

	gateway.Middleware(next).ServeHTTP(recorder, req)

	if !called || recorder.Code != http.StatusNoContent {
		t.Fatalf("called/status = %v/%d", called, recorder.Code)
	}
	if !strings.Contains(eval.key, PersistenceRedisPackage.APIGatewayIPRateKeyPrefix+"bilibili:") || strings.Contains(eval.key, "203.0.113.9") {
		t.Fatalf("unexpected rate key %q", eval.key)
	}
}

func TestHGAPIGatewayRejectsRateLimitAndRedisFailure(t *testing.T) {
	tests := []struct {
		name       string
		eval       *hgGatewayFakeEval
		wantStatus int
		wantCode   string
	}{
		{name: "rate limited", eval: &hgGatewayFakeEval{}, wantStatus: http.StatusTooManyRequests, wantCode: `"code": 100008`},
		{name: "redis failure", eval: &hgGatewayFakeEval{err: errors.New("redis down")}, wantStatus: http.StatusServiceUnavailable, wantCode: `"code": 500007`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gateway := hgTestGateway(t, test.eval)
			recorder := httptest.NewRecorder()
			gateway.Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Fatal("rejected request reached downstream")
			})).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil))
			if recorder.Code != test.wantStatus || !strings.Contains(recorder.Body.String(), test.wantCode) {
				t.Fatalf("status/body = %d/%s", recorder.Code, recorder.Body.String())
			}
			if recorder.Header().Get("X-Content-Type-Options") != "nosniff" || recorder.Header().Get("X-Request-ID") == "" {
				t.Fatalf("missing gateway headers: %#v", recorder.Header())
			}
		})
	}
}

func TestHGAPIGatewayTrustsForwardedIPOnlyFromConfiguredProxy(t *testing.T) {
	proxy := netip.MustParsePrefix("10.20.0.0/16")
	tests := []struct {
		name       string
		remoteAddr string
		wantIP     string
	}{
		{name: "trusted proxy", remoteAddr: "10.20.1.8:443", wantIP: "198.51.100.7"},
		{name: "untrusted direct client", remoteAddr: "203.0.113.9:443", wantIP: "203.0.113.9"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			eval := &hgGatewayFakeEval{allowed: 1}
			gateway := hgTestGateway(t, eval, proxy)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/profile/info", nil)
			req.RemoteAddr = test.remoteAddr
			req.Header.Set("X-Forwarded-For", "198.51.100.7")
			gateway.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })).ServeHTTP(httptest.NewRecorder(), req)
			wantKey := PersistenceRedisPackage.GetAPIGatewayIPRateKey("profile", test.wantIP)
			if eval.key != wantKey {
				t.Fatalf("rate key = %q, want %q", eval.key, wantKey)
			}
		})
	}
}

func TestHGAPIGatewayBypassesNonBusinessPaths(t *testing.T) {
	eval := &hgGatewayFakeEval{err: errors.New("must not be called")}
	gateway := hgTestGateway(t, eval)
	for _, path := range []string{"/api/v1/routes", "/uploads/avatar.png", "/healthz"} {
		recorder := httptest.NewRecorder()
		gateway.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusNoContent {
			t.Fatalf("path %s status = %d", path, recorder.Code)
		}
	}
}
