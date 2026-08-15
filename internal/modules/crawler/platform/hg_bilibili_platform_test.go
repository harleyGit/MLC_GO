package platform

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
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
