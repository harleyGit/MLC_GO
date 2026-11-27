package HGMiddlewarePackage

import (
	resp "MLC_GO/internal/response"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHGRecoverBusinessPanic(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Accept", resp.HGBusinessMediaType)
	RecoverMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("private SQL at internal.database") })).ServeHTTP(w, r)
	var b resp.HGBusinessEnvelope
	if json.Unmarshal(w.Body.Bytes(), &b) != nil || w.Code != 500 || b.Code != 1000001 || strings.Contains(w.Body.String(), "database") {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}

func TestHGRecoverDoesNotAppendAfterPartialResponse(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	defer func() {
		if recovered := recover(); recovered != http.ErrAbortHandler {
			t.Fatalf("部分响应必须中止连接：%v", recovered)
		}
		if w.Body.String() != "partial" {
			t.Fatal("已开始的响应被追加或覆盖")
		}
	}()
	RecoverMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("partial")); panic("failure") })).ServeHTTP(w, r)
	t.Fatal("部分响应 panic 没有中止")
}

func TestHGBusinessResponseOnlyCommitsOnce(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	tracked := &hgAccessLogResponseWriter{ResponseWriter: w}
	resp.HGSuccessStatus(tracked, r, 200, "first")
	first := w.Body.String()
	resp.HGFailStatus(tracked, r, 500, resp.InternalError)
	if w.Code != 200 || w.Body.String() != first {
		t.Fatal("已提交的响应被重复写入")
	}
}
