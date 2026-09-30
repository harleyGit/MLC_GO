package HGMiddlewarePackage

import (
	hgutils "MLC_GO/internal/pkg/utils"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

type hgFailingBody struct{}

func (hgFailingBody) Read([]byte) (int, error) { return 0, errors.New("test read failure") }
func (hgFailingBody) Close() error             { return nil }

// TestHGAPIGuardFailureResponse checks the entire response, not just its first JSON object.
func TestHGAPIGuardFailureResponse(t *testing.T) {
	for _, hgCase := range []struct {
		hgName, hgMessage string
		hgStatus, hgCode  int
		hgChange          func(*http.Request)
	}{
		{"signature mismatch", "signature无效", 401, 300001, func(hgReq *http.Request) {}},
		{"malformed signature", "signature无效", 401, 300001, func(hgReq *http.Request) { hgReq.Header.Set("X-Signature", "not-hex") }},
		{"missing headers", "请求头错误", 400, 100004, func(hgReq *http.Request) { hgReq.Header.Del("X-Device-ID") }},
		{"missing signature", "请求头错误", 400, 100004, func(hgReq *http.Request) { hgReq.Header.Del("X-Signature") }},
		{"missing authorization", "Authorization不能为空", 401, 300001, func(hgReq *http.Request) { hgReq.Header.Del("Authorization") }},
		{"expired timestamp", "timestamp无效或已过期", 401, 300001, func(hgReq *http.Request) { hgReq.Header.Set("X-Timestamp", "1") }},
		{"invalid timestamp", "timestamp无效或已过期", 401, 300001, func(hgReq *http.Request) { hgReq.Header.Set("X-Timestamp", "invalid") }},
		{"read failure", "请求体读取失败", 400, 100004, func(hgReq *http.Request) { hgReq.Body = hgFailingBody{} }},
		{"oversized body", "请求体读取失败", 400, 100004, func(hgReq *http.Request) {
			hgReq.Body = io.NopCloser(strings.NewReader("test"))
			hgReq.ContentLength = apiGuardMaxSignedBodyBytes + 1
		}},
	} {
		t.Run(hgCase.hgName, func(t *testing.T) {
			hgGuard := NewAPIGuard([]APIRule{{Path: "/recharge/skus", Version: "v1", Methods: map[string]bool{http.MethodGet: true}, NeedAuth: true}})
			hgReq := httptest.NewRequest(http.MethodGet, "/recharge/skus?pageSize=20&cursor=0", nil)
			hgReq = hgReq.WithContext(hgutils.InjectTID(hgReq.Context(), "hg-guard-test"))
			for hgKey, hgValue := range map[string]string{
				"Authorization": "Bearer test-invalid-token", "Content-Type": "application/json",
				"X-API-Version": "v1", "X-Device-ID": "test-device", "X-Client-Type": "web",
				"X-Client-Version": "1", "X-Language": "zh-CN", "X-Request-ID": "test-request",
				"X-Timestamp": strconv.FormatInt(time.Now().Unix(), 10), "X-Signature": "sha256=00",
			} {
				hgReq.Header.Set(hgKey, hgValue)
			}
			hgCase.hgChange(hgReq)
			hgRec := httptest.NewRecorder()
			hgGuard.Middleware()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Fatal("next handler should not be called")
			})).ServeHTTP(hgRec, hgReq)
			var hgResponse struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
				TID     string `json:"tid"`
			}
			if hgErr := json.Unmarshal(hgRec.Body.Bytes(), &hgResponse); hgErr != nil {
				t.Fatalf("expected exactly one JSON response: %v", hgErr)
			}
			if hgRec.Code != hgCase.hgStatus || hgResponse.Code != hgCase.hgCode || hgResponse.Message != hgCase.hgMessage || hgResponse.TID != "hg-guard-test" {
				t.Fatalf("status=%d response=%+v", hgRec.Code, hgResponse)
			}
		})
	}
}
