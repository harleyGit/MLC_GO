package OpsHandlerPackage

import (
	OpsDtoPackage "MLC_GO/internal/modules/ops/dto"
	OpsRepositoryPackage "MLC_GO/internal/modules/ops/repository"
	OpsServicePackage "MLC_GO/internal/modules/ops/service"
	UserJWTMiddlewarePackage "MLC_GO/internal/modules/user/middleware"
	UserServicePackage "MLC_GO/internal/modules/user/service"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestHGRechargeSKUListUsesResultEnvelope(t *testing.T) {
	hgDB, hgMock, hgErr := sqlmock.New()
	if hgErr != nil {
		t.Fatal(hgErr)
	}
	defer hgDB.Close()
	hgMock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectOpsAssetPermissionSQL)).WithArgs("admin", "payment.recharge_sku.read").WillReturnRows(sqlmock.NewRows([]string{"allowed"}).AddRow(1))
	hgMock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectOpsPaymentRechargeSKUListFirstSQL)).WithArgs(21).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	hgHandler := NewHandler(OpsServicePackage.NewService(OpsRepositoryPackage.NewRepository(hgDB), nil, nil))
	hgRequest := httptest.NewRequest(http.MethodGet, "/api/v1/ops/payment/recharge/skus/list", nil)
	hgRequest = hgRequest.WithContext(context.WithValue(hgRequest.Context(), UserJWTMiddlewarePackage.UserIDKey, &UserServicePackage.HGClaims{UserID: "admin"}))
	hgRecorder := httptest.NewRecorder()
	hgHandler.HGListRechargeSKUs(hgRecorder, hgRequest)
	var hgEnvelope map[string]json.RawMessage
	if hgErr := json.Unmarshal(hgRecorder.Body.Bytes(), &hgEnvelope); hgErr != nil {
		t.Fatal(hgErr)
	}
	var hgPage OpsDtoPackage.HGPaymentRechargeSKUListResponse
	if hgErr := json.Unmarshal(hgEnvelope["result"], &hgPage); hgErr != nil {
		t.Fatal(hgErr)
	}
	if hgRecorder.Code != http.StatusOK || hgPage.List == nil || len(hgPage.List) != 0 || hgPage.NextCursor != "" || hgPage.HasMore || hgPage.Total != -1 {
		t.Fatalf("response=%d %s", hgRecorder.Code, hgRecorder.Body.String())
	}
	if _, hgExists := hgEnvelope["data"]; hgExists {
		t.Fatal("public response must use result, not data")
	}
	if hgErr := hgMock.ExpectationsWereMet(); hgErr != nil {
		t.Fatal(hgErr)
	}
}

func TestHGRechargeSKUStrictBody(t *testing.T) {
	for _, body := range []string{`{"payAmount":1.1}`, `{"payAmount":"1"}`, `{"payAmount":1e2}`, `{"unknown":1}`, `{} {}`, `{"title":"` + strings.Repeat("x", 17<<10) + `"}`} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		var req OpsDtoPackage.HGPaymentRechargeSKUCreateRequest
		if hgDecodeRechargeSKU(w, r, &req) || w.Code != 400 {
			t.Fatalf("status=%d", w.Code)
		}
	}
}

func TestHGRechargeSKUAuthenticationAndErrors(t *testing.T) {
	h := NewHandler(nil)
	for _, handler := range []http.HandlerFunc{h.HGCreateRechargeSKU, h.HGUpdateRechargeSKU, h.HGDeleteRechargeSKU, h.HGListRechargeSKUs} {
		w := httptest.NewRecorder()
		handler(w, httptest.NewRequest(http.MethodPost, "/", nil))
		if w.Code != 401 {
			t.Fatalf("status=%d", w.Code)
		}
	}
	for _, tc := range []struct {
		err    error
		status int
	}{{OpsRepositoryPackage.ErrHGRechargeSKUConflict, 409}, {errors.New("database secret diagnostic"), 500}} {
		w := httptest.NewRecorder()
		hgWriteRechargeSKUError(w, httptest.NewRequest(http.MethodGet, "/", nil), tc.err)
		if w.Code != tc.status || strings.Contains(w.Body.String(), "secret") {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	}
}
