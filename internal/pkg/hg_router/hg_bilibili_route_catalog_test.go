package HGRouterPackage

import (
	"net/http"
	"testing"
)

func TestBilibiliRouteCatalog(t *testing.T) {
	t.Parallel()

	want := map[string]bool{
		"/api/v1/bilibili/author/profile":  false,
		"/api/v1/bilibili/author/stats":    false,
		"/api/v1/bilibili/author/videos":   false,
		"/api/v1/bilibili/author/homepage": false,
	}
	items := BilibiliRouteCatalog()
	if len(items) != len(want) {
		t.Fatalf("route count = %d, want %d", len(items), len(want))
	}
	for _, item := range items {
		needAuth, ok := want[item.Path]
		if !ok {
			t.Fatalf("unexpected route %q", item.Path)
		}
		if item.Method != http.MethodGet || item.NeedAuth != needAuth {
			t.Fatalf("route %q method/auth = %s/%v", item.Path, item.Method, item.NeedAuth)
		}
	}
}
