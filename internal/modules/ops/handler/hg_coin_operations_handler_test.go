package OpsHandlerPackage

import (
	OpsServicePackage "MLC_GO/internal/modules/ops/service"
	"net/http/httptest"
	"testing"
)

func TestHGAssetOperatorUsesRemoteAddrAndIgnoresForwardedBodyMetadata(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/v1/ops/coin/grant", nil)
	req.RemoteAddr = "203.0.113.8:43122"
	req.Header.Set("X-Forwarded-For", "198.51.100.9")
	operator := hgAssetOperator(req, "admin-1")
	if operator.ID != "admin-1" || operator.SourceIP != "203.0.113.8" {
		t.Fatalf("operator=%+v", operator)
	}
}

func TestHGWriteCoinOperationsErrorReturns429ForRateLimit(t *testing.T) {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/ops/coin/grant", nil)
	hgWriteCoinOperationsError(recorder, req, OpsServicePackage.ErrHGOperationsRateLimited)
	if recorder.Code != 429 {
		t.Fatalf("status=%d, want 429", recorder.Code)
	}
}
