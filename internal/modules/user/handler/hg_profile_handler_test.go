package UserHandlerPackage

import (
	UserJWTMiddlewarePackage "MLC_GO/internal/modules/user/middleware"
	UserServicePackage "MLC_GO/internal/modules/user/service"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseUpdateUserID_QueryKeepsBusinessUserID(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/profile/update?user_id=hgid_1001", nil)

	userID, err := parseUpdateUserID(req)
	if err != nil {
		t.Fatalf("parseUpdateUserID err=%v", err)
	}
	if userID != "hgid_1001" {
		t.Fatalf("parseUpdateUserID() = %q, want hgid_1001", userID)
	}
}

func TestParseUpdateUserID_ContextFallback(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/profile/update", nil)
	claims := &UserServicePackage.HGClaims{UserID: "hgid_from_token"}
	req = req.WithContext(contextWithUserClaims(req, claims))

	userID, err := parseUpdateUserID(req)
	if err != nil {
		t.Fatalf("parseUpdateUserID err=%v", err)
	}
	if userID != "hgid_from_token" {
		t.Fatalf("parseUpdateUserID() = %q, want hgid_from_token", userID)
	}
}

func TestPathUser_RejectsMissingUserID(t *testing.T) {
	h := NewUserHandler(HGUserHandlerDeps{})
	req := httptest.NewRequest(http.MethodPut, "/profile/path", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()

	h.PathUser(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("PathUser status=%d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func contextWithUserClaims(req *http.Request, claims *UserServicePackage.HGClaims) context.Context {
	return context.WithValue(req.Context(), UserJWTMiddlewarePackage.UserIDKey, claims)
}
