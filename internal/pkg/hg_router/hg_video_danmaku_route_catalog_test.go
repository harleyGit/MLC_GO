package HGRouterPackage

import "testing"

func TestVideoDanmakuRouteCatalog(t *testing.T) {
	want := map[string]bool{"/api/v1/video_danmaku/create": true, "/api/v1/video_danmaku/list": true, "/api/v1/video_danmaku/ticket": true, "/api/v1/video_danmaku/ws": true}
	for _, route := range VideoDanmakuRouteCatalog() {
		delete(want, route.Path)
		if !route.NeedAuth {
			t.Fatalf("danmaku route must require auth: %s", route.Path)
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing danmaku routes: %#v", want)
	}
}
