package platform

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

type hgRoundTripFunc func(*http.Request) (*http.Response, error)

func (f hgRoundTripFunc) Do(req *http.Request) (*http.Response, error) { return f(req) }

func TestHGBilibiliPlatformFetchRecommendations(t *testing.T) {
	client := hgRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Hostname() != "api.bilibili.com" || req.Header.Get("Referer") == "" {
			t.Fatalf("unexpected request: %s", req.URL.String())
		}
		body := `{"code":0,"message":"OK","data":{"item":[` +
			`{"bvid":"BV1","goto":"av","uri":"https://www.bilibili.com/video/BV1","pic":"http://i0.hdslb.com/a.jpg","title":" video ","duration":10,"pubdate":1700000000,"owner":{"mid":9,"name":"author"},"stat":{"view":100,"like":10,"danmaku":2}},` +
			`{"bvid":"BV1","goto":"av","title":"duplicate"},` +
			`{"bvid":"LIVE","goto":"live","title":"live"}` +
			`]}}`
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	platform, err := NewHGBilibiliPlatform(client, HGBilibiliConfig{MaxItems: 10, RetryCount: 0, RatePerSecond: 1})
	if err != nil {
		t.Fatalf("NewHGBilibiliPlatform() error = %v", err)
	}
	items, err := platform.FetchRecommendations(context.Background())
	if err != nil {
		t.Fatalf("FetchRecommendations() error = %v", err)
	}
	if len(items) != 1 || items[0].ContentID != "BV1" {
		t.Fatalf("items = %#v", items)
	}
	if items[0].CoverURL != "https://i0.hdslb.com/a.jpg" || items[0].Title != "video" {
		t.Fatalf("normalized item = %#v", items[0])
	}
}

func TestHGBilibiliPlatformRejectsBusinessError(t *testing.T) {
	client := hgRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"code":-412,"message":"request blocked"}`)), Header: make(http.Header)}, nil
	})
	platform, err := NewHGBilibiliPlatform(client, HGBilibiliConfig{RetryCount: 0, RatePerSecond: 1, RequestTimeout: time.Second})
	if err != nil {
		t.Fatalf("NewHGBilibiliPlatform() error = %v", err)
	}
	if _, err := platform.FetchRecommendations(context.Background()); err == nil || !strings.Contains(err.Error(), "business code -412") {
		t.Fatalf("FetchRecommendations() error = %v", err)
	}
}

func TestNewHGBilibiliPlatformRejectsUntrustedEndpoint(t *testing.T) {
	_, err := NewHGBilibiliPlatform(nil, HGBilibiliConfig{Endpoint: "https://example.com/crawler"})
	if err == nil {
		t.Fatal("NewHGBilibiliPlatform() expected endpoint validation error")
	}
}

func TestHGBilibiliPlatformRateLimitsEveryRetry(t *testing.T) {
	var calls atomic.Int32
	client := hgRoundTripFunc(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{StatusCode: http.StatusBadGateway, Body: io.NopCloser(strings.NewReader("upstream unavailable")), Header: make(http.Header)}, nil
	})
	platform, err := NewHGBilibiliPlatform(client, HGBilibiliConfig{RetryCount: 2, RatePerSecond: 1, RequestTimeout: time.Second})
	if err != nil {
		t.Fatalf("NewHGBilibiliPlatform() error = %v", err)
	}
	// 首个令牌允许第一次请求立即执行；后续令牌故意设置为一小时后，用短 context 验证重试不会绕过限流器。
	platform.limiter = rate.NewLimiter(rate.Every(time.Hour), 1)
	ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
	defer cancel()

	if _, err := platform.FetchRecommendations(ctx); err == nil || !strings.Contains(err.Error(), "rate limiter") {
		t.Fatalf("FetchRecommendations() error = %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("HTTP calls = %d, want 1 because retry must wait for another rate-limit token", got)
	}
}
