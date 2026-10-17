package HGRouterPackage

import (
	hgHandler "MLC_GO/internal/modules/video_upload/handler"
	hgServer "MLC_GO/internal/pkg/server"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHGVideoGuardParity 防止已注册视频接口遗漏独立 Guard 规则。
func TestHGVideoGuardParity(t *testing.T) {
	for _, hgRoute := range videoUploadRoutes(nil) {
		hgFound := false
		for _, hgRule := range hgServer.VideoUploadMethodRules() {
			if hgRule.Path == hgRoute.SubPath && hgRule.Version == "v1" && hgRule.Methods[hgRoute.Method] && hgRule.NeedAuth == hgRoute.NeedAuth {
				hgFound = true
			}
		}
		if !hgFound {
			t.Errorf("接口规则缺失: %s", hgRoute.FullPath)
		}
	}
}

// TestHGVideoDetailGuardChain 使用真实路由组验证详情请求进入鉴权检查而非404。
func TestHGVideoDetailGuardChain(t *testing.T) {
	hgRoot := http.NewServeMux()
	hgRoot.Handle(VideoUploadModuleBasePath+"/", http.StripPrefix(VideoUploadModuleBasePath, NewVideoUploadRouteGroup(&hgHandler.Handler{})))
	hgReq := httptest.NewRequest(http.MethodGet, VideoUploadModuleBasePath+"/detail?contentId=external%3Abilibili%3ABV1zVJY63EcT", nil)
	hgRec := httptest.NewRecorder()
	hgRoot.ServeHTTP(hgRec, hgReq)
	if hgRec.Code != http.StatusBadRequest {
		t.Fatalf("缺少公共请求头应为400，实际%d: %s", hgRec.Code, hgRec.Body.String())
	}
}
