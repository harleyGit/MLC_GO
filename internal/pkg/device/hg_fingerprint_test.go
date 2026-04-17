package PkGDevicePackage

import (
	"net/http/httptest"
	"testing"
)

func TestFingerprint_UsesDeviceIDWhenPresent(t *testing.T) {
	req1 := httptest.NewRequest("GET", "http://example.com/api/v1/profile/list", nil)
	req1.RemoteAddr = "127.0.0.1:51001"
	req1.Header.Set("X-Device-ID", "web-device-001")
	req1.Header.Set("X-Client-Type", "web")
	req1.Header.Set("X-Language", "zh-CN")
	req1.Header.Set("User-Agent", "Mozilla/5.0 test")

	req2 := httptest.NewRequest("GET", "http://example.com/api/v1/profile/list", nil)
	req2.RemoteAddr = "127.0.0.1:52099"
	req2.Header.Set("X-Device-ID", "web-device-001")
	req2.Header.Set("X-Client-Type", "web")
	req2.Header.Set("X-Language", "zh-CN")
	req2.Header.Set("User-Agent", "Mozilla/5.0 test")

	fp1 := Fingerprint(req1)
	fp2 := Fingerprint(req2)
	if fp1 != fp2 {
		t.Fatalf("fingerprint should be stable when device-id is same, got %s != %s", fp1, fp2)
	}
}

func TestFingerprint_FallbackIgnoresRemotePort(t *testing.T) {
	req1 := httptest.NewRequest("GET", "http://example.com/api/v1/profile/list", nil)
	req1.RemoteAddr = "10.0.0.8:10001"
	req1.Header.Set("X-Client-Type", "web")
	req1.Header.Set("Accept-Language", "zh-CN")
	req1.Header.Set("User-Agent", "Mozilla/5.0 test")

	req2 := httptest.NewRequest("GET", "http://example.com/api/v1/profile/list", nil)
	req2.RemoteAddr = "10.0.0.8:59999"
	req2.Header.Set("X-Client-Type", "web")
	req2.Header.Set("Accept-Language", "zh-CN")
	req2.Header.Set("User-Agent", "Mozilla/5.0 test")

	fp1 := Fingerprint(req1)
	fp2 := Fingerprint(req2)
	if fp1 != fp2 {
		t.Fatalf("fallback fingerprint should ignore remote port, got %s != %s", fp1, fp2)
	}
}

func TestExtractClientIP_PrefersForwardedHeaders(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/api/v1/profile/list", nil)
	req.RemoteAddr = "127.0.0.1:52341"
	req.Header.Set("X-Forwarded-For", "203.0.113.10, 10.0.0.1")

	ip := extractClientIP(req)
	if ip != "203.0.113.10" {
		t.Fatalf("expected forwarded ip, got %s", ip)
	}
}
