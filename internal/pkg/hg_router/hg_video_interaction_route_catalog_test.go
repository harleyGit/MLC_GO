package HGRouterPackage

import "testing"

func TestVideoInteractionRouteCatalogContainsPageActions(t *testing.T) {
	want := map[string]bool{
		"GET /api/v1/video_interactions/state":     false,
		"POST /api/v1/video_interactions/like":     false,
		"POST /api/v1/video_interactions/coin":     false,
		"POST /api/v1/video_interactions/favorite": false,
		"POST /api/v1/video_interactions/share":    false,
		"POST /api/v1/video_interactions/follow":   false,
	}
	for _, item := range VideoInteractionRouteCatalog() {
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
