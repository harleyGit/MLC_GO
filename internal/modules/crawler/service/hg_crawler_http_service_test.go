package service

import (
	CrawlerDtoPackage "MLC_GO/internal/modules/crawler/dto"
	"errors"
	"strings"
	"testing"
)

func TestHGTargetPolicyUsesExactHostsAndHTTPSByDefault(t *testing.T) {
	policy, err := NewHGTargetPolicy([]string{"Example.COM."}, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := policy.ValidateTarget("https://example.com/feed"); err != nil {
		t.Fatalf("allowed target error = %v", err)
	}
	for _, target := range []string{"https://sub.example.com/feed", "http://example.com/feed", "https://example.com.evil.test/feed"} {
		if _, err := policy.ValidateTarget(target); !errors.Is(err, ErrHGCrawlerUnsafeTarget) {
			t.Fatalf("target %s error = %v, want unsafe target", target, err)
		}
	}
}

func TestHGTargetPolicyRejectsPrivateLiteralIP(t *testing.T) {
	for _, host := range []string{"127.0.0.1", "192.0.2.1", "2001:db8::1"} {
		policy, err := NewHGTargetPolicy([]string{host}, true)
		if err != nil {
			t.Fatal(err)
		}
		target := "http://" + host + "/feed"
		if strings.Contains(host, ":") {
			target = "http://[" + host + "]/feed"
		}
		if _, err := policy.ValidateTarget(target); !errors.Is(err, ErrHGCrawlerUnsafeTarget) {
			t.Fatalf("non-public literal IP %s error = %v", host, err)
		}
	}
}

func TestNormalizeBilibiliVideoURLUsesAllowlistedDetailAPI(t *testing.T) {
	got, err := hgNormalizeBilibiliVideoURL("https://www.bilibili.com/video/BV1XP8u6xEwk/?spm_id_from=333")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://api.bilibili.com/x/web-interface/view?bvid=BV1XP8u6xEwk" {
		t.Fatalf("normalized URL = %q", got)
	}
	if _, err := hgNormalizeBilibiliVideoURL("https://www.bilibili.com/read/cv1"); !errors.Is(err, ErrHGCrawlerInvalidRequest) {
		t.Fatalf("invalid Bilibili URL error = %v", err)
	}
}

func TestBilibiliVideoURLPassesRequestValidationAfterNormalization(t *testing.T) {
	policy, err := NewHGTargetPolicy([]string{"api.bilibili.com"}, false)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewHGSafeHTTPService(policy, "test-agent")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.ValidateRequest(CrawlerDtoPackage.HGDebugRequest{
		URL:    "https://www.bilibili.com/video/BV1XP8u6xEwk/?spm_id_from=333",
		Method: "GET",
	}); err != nil {
		t.Fatalf("ValidateRequest() error = %v", err)
	}
}
