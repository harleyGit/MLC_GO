package HGServerPackage

import (
	"net/http"
	"testing"
)

func TestVideoCommentMethodRulesContainNewAuthenticatedEndpoints(t *testing.T) {
	want := map[string]string{"/replies": http.MethodGet, "/reaction": http.MethodPost, "/image": http.MethodPost}
	for _, rule := range VideoCommentMethodRules() {
		method, ok := want[rule.Path]
		if ok && rule.NeedAuth && rule.Methods[method] {
			delete(want, rule.Path)
		}
	}
	for path := range want {
		t.Fatalf("API Guard rule missing authenticated endpoint %s", path)
	}
}
