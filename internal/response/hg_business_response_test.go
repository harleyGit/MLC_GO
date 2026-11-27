package HGResponsePakcage

import (
	"MLC_GO/internal/response/hgerrors"
	"encoding/json"
	"errors"
	"go.opentelemetry.io/otel/trace"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHGBusinessHTTPContract(t *testing.T) {
	for _, tc := range []struct {
		err          error
		status, code int
		key          string
	}{
		{hgerrors.New(hgerrors.HGAuthTokenExpired), 401, 1101002, "AUTH_TOKEN_EXPIRED"},
		{hgerrors.New(hgerrors.HGIAMPermissionDenied), 403, 1201003, "IAM_PERMISSION_DENIED"},
		{hgerrors.New(hgerrors.HGMallStockInsufficient), 409, 1402004, "MALL_STOCK_INSUFFICIENT"},
		{errors.New("SELECT private FROM db.internal"), 500, 1000001, "COMMON_INTERNAL_ERROR"},
	} {
		req := httptest.NewRequest("GET", "/", nil)
		sc := trace.NewSpanContext(trace.SpanContextConfig{TraceID: trace.TraceID{1}, SpanID: trace.SpanID{2}})
		req = req.WithContext(trace.ContextWithSpanContext(req.Context(), sc))
		req.Header.Set("Accept", HGBusinessMediaType)
		w := httptest.NewRecorder()
		HGFailError(w, req, 200, InternalError, tc.err)
		var b map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &b); err != nil {
			t.Fatal(err)
		}
		if data, ok := b["data"]; !ok || data != nil || b["error"] != tc.key || b["code"] != float64(tc.code) || b["trace_id"] != sc.TraceID().String() || w.Code != tc.status || strings.Contains(w.Body.String(), "db.internal") {
			t.Fatalf("HTTP %d %v", w.Code, b)
		}
	}
}

func TestHGPageNegotiation(t *testing.T) {
	page := NewPageResponse([]string{"a"}, WithTotal(1))
	for _, v2 := range []bool{false, true} {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		key := "result"
		if v2 {
			req.Header.Set("Accept", HGBusinessMediaType)
			key = "data"
		}
		SuccessPageResult(w, req, page)
		var b map[string]json.RawMessage
		var got HGPageResultModel[string]
		if json.Unmarshal(w.Body.Bytes(), &b) != nil || json.Unmarshal(b[key], &got) != nil || got.Seid != page.Seid || got.NumResults != 1 || len(got.Result) != 1 || got.Result[0] != "a" {
			t.Fatalf("分页协议丢失：%s", w.Body.String())
		}
		if string(b["code"]) != "200" || w.Result().Header.Get("Vary") != "Accept" {
			t.Fatal("成功码或缓存协商错误")
		}
	}
}

func TestHGAcceptedAndEncodingFailure(t *testing.T) {
	for _, accepted := range []bool{true, false} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/", nil)
		r.Header.Set("Accept", HGBusinessMediaType)
		if accepted {
			HGSuccessStatus(w, r, 202, map[string]string{"status": "accepted"})
		} else {
			HGSuccessStatus(w, r, 200, make(chan int))
		}
		var b HGBusinessEnvelope
		if json.Unmarshal(w.Body.Bytes(), &b) != nil {
			t.Fatal("输出不是单个 JSON")
		}
		if accepted {
			if w.Code != 202 || b.Code != 200 || b.Data.(map[string]any)["status"] != "accepted" {
				t.Fatalf("%d %+v", w.Code, b)
			}
		} else if w.Code != 500 || b.Code != 1000001 || b.Data != nil {
			t.Fatalf("%d %+v", w.Code, b)
		}
	}
}
