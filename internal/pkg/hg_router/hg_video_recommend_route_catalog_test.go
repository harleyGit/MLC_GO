package HGRouterPackage

import (
	"net/http"
	"testing"
)

func TestVideoRecommendRouteCatalog(t *testing.T) {
	items := VideoRecommendRouteCatalog()
	if len(items) != 1 || items[0].Path != "/api/v1/video_recommend/feed" || items[0].Method != http.MethodGet || !items[0].NeedAuth {
		t.Fatalf("unexpected routes: %#v", items)
	}
}
