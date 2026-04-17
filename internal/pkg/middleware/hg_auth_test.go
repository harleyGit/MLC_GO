package PkgMiddlewarePackage

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthMiddleware_MissingTokenReturnStandardError(t *testing.T) {
	handler := AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("next handler should not be called")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/profile/info", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status=%d, got=%d", http.StatusUnauthorized, rec.Code)
	}
	assertTokenInvalidResponse(t, rec.Body.Bytes())
}

func TestTokenAuthMiddleware_MissingTokenReturnStandardError(t *testing.T) {
	handler := TokenAuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("next handler should not be called")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/profile/info", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status=%d, got=%d", http.StatusUnauthorized, rec.Code)
	}
	assertTokenInvalidResponse(t, rec.Body.Bytes())
}

func assertTokenInvalidResponse(t *testing.T, body []byte) {
	t.Helper()

	var resp struct {
		Code      int    `json:"code"`
		Message   string `json:"message"`
		TID       string `json:"tid"`
		Timestamp int64  `json:"timestamp"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal response failed: %v, body=%s", err, string(body))
	}

	if resp.Code != 404003 {
		t.Fatalf("expected code=404003, got=%d, body=%s", resp.Code, string(body))
	}
	if resp.Message != "Token无效" {
		t.Fatalf("expected message=Token无效, got=%s, body=%s", resp.Message, string(body))
	}
	if resp.Timestamp <= 0 {
		t.Fatalf("expected timestamp > 0, body=%s", string(body))
	}
}
