package VideoCommentHandlerPackage

import (
	VideoCommentServicePackage "MLC_GO/internal/modules/video_comment/service"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

func TestDeleteRootWithRepliesReturnsConflict(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/video_comments/delete", nil)
	hgWriteError(recorder, request, VideoCommentServicePackage.ErrCommentHasReplies)

	if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), "存在回复的评论不能删除") {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestHGClientIPIgnoresForwardedHeadersFromUntrustedPeer(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/video_comments/image", nil)
	request.RemoteAddr = "203.0.113.8:43122"
	request.Header.Set("X-Forwarded-For", "198.51.100.9")

	if got := hgClientIP(request, []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}); got != "203.0.113.8" {
		t.Fatalf("hgClientIP() = %q, want direct peer", got)
	}
}

func TestHGClientIPRemovesTrustedProxyChainFromRight(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/video_comments/image", nil)
	request.RemoteAddr = "10.0.0.8:43122"
	request.Header.Set("X-Forwarded-For", "198.51.100.9, 10.1.0.7")

	trusted := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	if got := hgClientIP(request, trusted); got != "198.51.100.9" {
		t.Fatalf("hgClientIP() = %q, want original client", got)
	}
}

func TestHGClientIPUsesRealIPOnlyFromTrustedPeer(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/video_comments/image", nil)
	request.RemoteAddr = "10.0.0.8:43122"
	request.Header.Set("X-Real-IP", "198.51.100.9")

	trusted := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	if got := hgClientIP(request, trusted); got != "198.51.100.9" {
		t.Fatalf("hgClientIP() = %q, want X-Real-IP", got)
	}
}

func TestHGClientIPFallsBackToPeerForMalformedForwardedChain(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/video_comments/image", nil)
	request.RemoteAddr = "10.0.0.8:43122"
	request.Header.Set("X-Forwarded-For", "198.51.100.9, malformed")

	trusted := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	if got := hgClientIP(request, trusted); got != "10.0.0.8" {
		t.Fatalf("hgClientIP() = %q, want safe peer fallback", got)
	}
}
