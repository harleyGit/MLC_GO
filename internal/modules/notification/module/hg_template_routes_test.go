package module

import (
	"net/http"
	"net/http/httptest"
	"testing"

	router "MLC_GO/internal/pkg/hg_router"
	permission "MLC_GO/internal/pkg/permission"
)

func TestHGTemplateRoutesRequireIAM(t *testing.T) {
	routes := hgTemplateRoutes(nil)
	expected := map[string]permission.HGPermission{
		http.MethodGet + " /templates":           permission.HGNotificationTemplateRead,
		http.MethodGet + " /templates/published": permission.HGNotificationTemplateRead,
		http.MethodGet + " /templates/detail":    permission.HGNotificationTemplateRead,
		http.MethodPost + " /templates":          permission.HGNotificationTemplateCreate,
		http.MethodPut + " /templates/update":    permission.HGNotificationTemplateUpdate,
		http.MethodPost + " /templates/delete":   permission.HGNotificationTemplateDelete,
		http.MethodPost + " /templates/publish":  permission.HGNotificationTemplateUpdate,
	}
	if len(routes) != len(expected) {
		t.Fatalf("路由数不匹配: %d", len(routes))
	}
	for _, route := range routes {
		key := route.Method + " " + route.SubPath
		if !route.NeedAuth || len(route.Permissions) != 1 || route.Permissions[0] != expected[key] || route.Handler != nil {
			t.Fatalf("模板路由权限配置错误: %s", key)
		}
	}
}

// 固定路径必须能穿过精确路径 API Guard；错误头先返回 400，而非路由漏匹配的 404。
func TestHGTemplateDetailGuardPath(t *testing.T) {
	base := "/api/v1/admin/notification"
	group := router.NewRouteGroup(router.RouteGroupConfig{BasePath: base}, hgTemplateRoutes(nil))
	request := httptest.NewRequest(http.MethodGet, base+"/templates/detail?id=1", nil)
	request.URL.Path = "/templates/detail"
	response := httptest.NewRecorder()
	group.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("模板详情未进入精确路径 Guard: %d", response.Code)
	}
}

func TestHGSendRoutesRequireSeparateIAM(t *testing.T) {
	expected := map[string]permission.HGPermission{
		"POST /templates/publish-send":  permission.HGNotificationTemplatePublish,
		"POST /send-tasks":              permission.HGNotificationSendCreate,
		"GET /send-tasks":               permission.HGNotificationSendRead,
		"GET /send-tasks/detail":        permission.HGNotificationSendRead,
		"POST /send-tasks/cancel":       permission.HGNotificationSendCancel,
		"GET /send-tasks/review":        permission.HGNotificationSendApprove,
		"GET /send-tasks/review/detail": permission.HGNotificationSendApprove,
		"POST /send-tasks/approve":      permission.HGNotificationSendApprove,
	}
	for _, route := range hgSendRoutes(nil) {
		key := route.Method + " " + route.SubPath
		if !route.NeedAuth || len(route.Permissions) != 1 || route.Permissions[0] != expected[key] || route.Handler != nil {
			t.Fatalf("发送路由权限配置错误: %s", key)
		}
		delete(expected, key)
	}
	if len(expected) != 0 {
		t.Fatalf("缺少发送路由: %v", expected)
	}
}
