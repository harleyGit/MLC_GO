package HGRouterPackage

import (
	VideoDanmakuHandlerPackage "MLC_GO/internal/modules/video_danmaku/handler"
	HGPermissionPackage "MLC_GO/internal/pkg/permission"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVideoDanmakuRouteCatalog(t *testing.T) {
	want := map[string]bool{"/api/v1/video_danmaku/create": true, "/api/v1/video_danmaku/list": true, "/api/v1/video_danmaku/ticket": true, "/api/v1/video_danmaku/ws": true}
	for _, route := range VideoDanmakuRouteCatalog() {
		delete(want, route.Path)
		if !route.NeedAuth {
			t.Fatalf("danmaku route must require auth: %s", route.Path)
		}
		if (route.Binding == HGRouteBindingExternal) != (route.Path == VideoDanmakuModuleBasePath+"/ws") {
			t.Fatalf("弹幕 HTTP 与独立 WebSocket 绑定类型错误: %+v", route)
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing danmaku routes: %#v", want)
	}
}

// 标准 HTTP 服务必须成功装配且不能接管独立 gnet 入口；票据认证仍由 gnet 服务负责。
func TestHGDanmakuExternalRouteDoesNotBindHTTP(t *testing.T) {
	hgHandler := &VideoDanmakuHandlerPackage.Handler{}
	hgRoot := http.NewServeMux()
	hgRoot.Handle(VideoDanmakuModuleBasePath+"/", http.StripPrefix(VideoDanmakuModuleBasePath, NewVideoDanmakuRouteGroup(hgHandler)))
	hgMux := http.NewServeMux()
	BindRouteSpecs(hgMux, videoDanmakuRoutes(hgHandler))
	for _, hgPath := range []string{"/create", "/list", "/ticket", "/ws"} {
		hgMethod := http.MethodPost
		if hgPath == "/list" || hgPath == "/ws" {
			hgMethod = http.MethodGet
		}
		hgResponse := httptest.NewRecorder()
		hgRoot.ServeHTTP(hgResponse, httptest.NewRequest(hgMethod, VideoDanmakuModuleBasePath+hgPath, nil))
		hgWant := http.StatusBadRequest // HTTP 路由应先命中公共请求头检查。
		if hgPath == "/ws" {
			hgWant = http.StatusNotFound
		}
		if hgResponse.Code != hgWant {
			t.Fatalf("%s 状态=%d，预期=%d", hgPath, hgResponse.Code, hgWant)
		}
		_, hgPattern := hgMux.Handler(httptest.NewRequest(hgMethod, hgPath, nil))
		if (hgPattern == "") != (hgPath == "/ws") {
			t.Fatalf("HTTP mux 绑定错误: %s -> %s", hgPath, hgPattern)
		}
	}
}

func TestHGExternalRouteRejectsHTTPHandlerAndPermissions(t *testing.T) {
	for _, hgCase := range []string{"handler", "permissions", "unknown_binding"} {
		t.Run(hgCase, func(t *testing.T) {
			hgSpec := NewHGExternalRouteSpec("test", http.MethodGet, "/test", "/ws", true, "独立入口")
			switch hgCase {
			case "handler":
				hgSpec.Handler = func(http.ResponseWriter, *http.Request) {}
			case "permissions":
				hgSpec.Permissions = []HGPermissionPackage.HGPermission{"iam:role:read"}
			case "unknown_binding":
				hgSpec.Binding = "unknown"
			}
			defer func() {
				if recover() == nil {
					t.Fatal("独立目录项错误配置未被拒绝")
				}
			}()
			NewRouteGroup(RouteGroupConfig{}, []RouteSpec{hgSpec})
		})
	}
}
