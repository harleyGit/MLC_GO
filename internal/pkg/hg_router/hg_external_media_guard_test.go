package HGRouterPackage

import (
	hgHandler "MLC_GO/internal/modules/video_upload/handler"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHGExternalMediaGuardChain 验证真实StripPrefix路由：播放凭证走Guard，媒体无凭证不能访问。
func TestHGExternalMediaGuardChain(t *testing.T) {
	t.Setenv("SERVER_ENV", "debug")
	found := false
	for _, rule := range hgRouteGuardRules(videoUploadRoutes(nil)) {
		if rule.Path == "/external-playback" && rule.Methods[http.MethodPost] && rule.NeedAuth {
			found = true
		}
	}
	if !found {
		t.Fatal("缺少播放凭证Guard规则")
	}
	mux := http.NewServeMux()
	mux.Handle(VideoUploadModuleBasePath+"/", http.StripPrefix(VideoUploadModuleBasePath, NewVideoUploadRouteGroup(hgHandler.NewHandler(nil))))
	for _, tc := range []struct {
		path, method string
		status       int
	}{{"/external-stream", "GET", 401}, {"/external-stream", "POST", 405}, {"/external-playback", "POST", 400}} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(tc.method, VideoUploadModuleBasePath+tc.path, nil))
		if w.Code != tc.status {
			t.Fatalf("%s: %d != %d", tc.path, w.Code, tc.status)
		}
	}
}
