package UserHandlerPackage

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	UserServicePackage "MLC_GO/internal/modules/user/service"
)

func TestHGAddressHandlerRejectsUnauthenticated(t *testing.T) {
	h := NewUserHandler(HGUserHandlerDeps{})
	for _, tc := range []struct {
		method, path string
		run          func(http.ResponseWriter, *http.Request)
	}{
		{http.MethodGet, "/addresses", h.HGListAddresses},
		{http.MethodPost, "/addresses", h.HGCreateAddress},
		{http.MethodGet, "/addresses/detail?id=1", h.HGGetAddress},
		{http.MethodPut, "/addresses/detail?id=1", h.HGUpdateAddress},
		{http.MethodDelete, "/addresses/detail?id=1", h.HGDeleteAddress},
		{http.MethodPut, "/addresses/default?id=1", h.HGSetDefaultAddress},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		resp := httptest.NewRecorder()
		tc.run(resp, req)
		if resp.Code != http.StatusUnauthorized {
			t.Errorf("%s %s 返回 %d", tc.method, tc.path, resp.Code)
		}
	}
}

func TestHGAddressHandlerRejectsInvalidIDAndBody(t *testing.T) {
	h := NewUserHandler(HGUserHandlerDeps{HGAddressService: &UserServicePackage.HGAddressService{}})
	for _, body := range []string{`{"receiverName":"张三","unexpected":1}`, strings.Repeat("x", 5000), `{} {}`} {
		req := httptest.NewRequest(http.MethodPost, "/addresses", strings.NewReader(body))
		req = req.WithContext(contextWithUserClaims(req, &UserServicePackage.HGClaims{UserID: "owner"}))
		resp := httptest.NewRecorder()
		h.HGCreateAddress(resp, req)
		if resp.Code != http.StatusBadRequest {
			t.Errorf("非法请求返回 %d", resp.Code)
		}
	}
	req := httptest.NewRequest(http.MethodDelete, "/addresses/detail?id=not-a-number", nil)
	req = req.WithContext(contextWithUserClaims(req, &UserServicePackage.HGClaims{UserID: "owner"}))
	resp := httptest.NewRecorder()
	h.HGDeleteAddress(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Errorf("非法 ID 返回 %d", resp.Code)
	}
}
