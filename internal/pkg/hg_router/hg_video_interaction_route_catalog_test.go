package HGRouterPackage

import (
	HGServerPackage "MLC_GO/internal/pkg/server"
	"net/http"
	"testing"
)

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

func TestVideoInteractionFollowRuleMatchesRegisteredRoute(t *testing.T) {
	const followPath = "/follow"
	foundRule := false
	for _, rule := range HGServerPackage.VideoInteractionMethodRules() {
		if rule.Path != followPath {
			continue
		}
		foundRule = true
		if !rule.NeedAuth {
			t.Fatal("follow API rule must require authentication")
		}
		if !rule.Methods[http.MethodPost] {
			t.Fatal("follow API rule must allow POST")
		}
	}
	if !foundRule {
		t.Fatal("video interaction API rules missing /follow")
	}

	foundRoute := false
	for _, route := range videoInteractionRoutes(nil) {
		if route.SubPath == followPath && route.Method == http.MethodPost {
			foundRoute = true
			break
		}
	}
	if !foundRoute {
		t.Fatal("video interaction routes missing POST /follow")
	}
}
