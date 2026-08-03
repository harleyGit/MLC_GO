package HGRouterPackage

import "testing"

func TestVideoCommentRouteCatalogContainsAuthenticatedEndpoints(t *testing.T) {
	want := map[string]bool{
		"POST /api/v1/video_comments/create": false,
		"GET /api/v1/video_comments/list":    false,
		"POST /api/v1/video_comments/delete": false,
	}
	for _, item := range VideoCommentRouteCatalog() {
		key := item.Method + " " + item.Path
		if _, ok := want[key]; ok {
			want[key] = true
		}
	}
	for route, found := range want {
		if !found {
			t.Fatalf("route catalog missing %s", route)
		}
	}
}
