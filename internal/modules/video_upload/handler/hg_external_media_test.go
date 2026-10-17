package VideoUploadHandlerPackage

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type hgMediaTransport func(*http.Request) (*http.Response, error)

func (h hgMediaTransport) RoundTrip(r *http.Request) (*http.Response, error) { return h(r) }

// TestHGMediaBoundary 覆盖凭证、Range上限及上游安全域名，测试不访问真实网络。
func TestHGMediaBoundary(t *testing.T) {
	for _, raw := range []string{"http://a.bilivideo.com/x.mp4", "https://bilivideo.com.evil.test/x.mp4", "https://127.0.0.1/x.mp4", "https://user@a.bilivideo.com/x.mp4", "https://a.bilivideo.com:4483/x.mp4"} {
		if hgAllowedMediaURL(raw) {
			t.Fatalf("接受非法源 %s", raw)
		}
	}
	if !hgAllowedMediaURL("https://a.bilivideo.com/x.mp4?deadline=123") {
		t.Fatal("拒绝合法源")
	}
	t.Setenv("SERVER_ENV", "debug")
	h := NewHandler(nil)
	id := "external:bilibili:BV1zVJY63EcT:42806413546"
	h.hgMedia.hgSessions["test-session"] = hgMediaSession{id, "https://a.bilivideo.com/x.mp4", time.Now().Add(time.Minute)}
	calls := 0
	h.hgMedia.hgClient = &http.Client{Transport: hgMediaTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Header.Get("Range") != "bytes=100-8388707" {
			t.Fatalf("Range未限界: %s", r.Header.Get("Range"))
		}
		if r.Header.Get("Referer") != "https://www.bilibili.com/" {
			t.Fatal("缺少媒体来源头")
		}
		return &http.Response{StatusCode: 206, ContentLength: 4, Header: http.Header{"Content-Type": {"video/mp4"}, "Content-Range": {"bytes 100-103/1000"}}, Body: io.NopCloser(strings.NewReader("test"))}, nil
	})}
	for _, tc := range []struct {
		cookie, rangeValue string
		status             int
	}{{"", "bytes=100-", 401}, {"test-session", "bytes=1-2,3-4", 416}, {"test-session", "bytes=100-", 206}} {
		r := httptest.NewRequest("GET", "/external-stream?videoId="+id, nil)
		r.Header.Set("Range", tc.rangeValue)
		if tc.cookie != "" {
			r.AddCookie(&http.Cookie{Name: hgMediaCookieName(id), Value: tc.cookie})
		}
		w := httptest.NewRecorder()
		h.HGExternalStream(w, r)
		if w.Code != tc.status {
			t.Fatalf("状态=%d,期望=%d", w.Code, tc.status)
		}
	}
	if calls != 1 {
		t.Fatalf("未鉴权请求访问了上游: %d", calls)
	}
	h.hgMedia.hgSessions["test-session"] = hgMediaSession{id, "", time.Now().Add(-time.Second)}
	r := httptest.NewRequest("GET", "/external-stream?videoId="+id, nil)
	r.AddCookie(&http.Cookie{Name: hgMediaCookieName(id), Value: "test-session"})
	w := httptest.NewRecorder()
	h.HGExternalStream(w, r)
	if w.Code != 401 {
		t.Fatal("过期凭证被接受")
	}
}
