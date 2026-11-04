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

func TestUpdateSecurity_AdminCannotChangeOtherSecrets(t *testing.T) {
	for _, body := range []string{
		`{"password":"secret"}`,
		`{"phone":"19900000000","qq":"12345"}`,
		`{"wechat":"other"}`,
		`{}`,
	} {
		req := httptest.NewRequest(http.MethodPut, "/security?user_id=target", strings.NewReader(body))
		req = req.WithContext(context.WithValue(req.Context(), UserJWTMiddlewarePackage.UserIDKey, &UserServicePackage.HGClaims{UserID: "admin"}))
		rec := httptest.NewRecorder()
		NewUserHandler(HGUserHandlerDeps{}).UpdateSecurity(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("管理员代改请求 %s 应被拒绝，实际状态 %d", body, rec.Code)
		}
	}
}
