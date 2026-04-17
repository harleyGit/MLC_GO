package HGTestHandlerPackage

import (
	HGMiddlewarePackage "MLC_GO/internal/interfaces/middleware"
	HGMiddlewareGroupPackage "MLC_GO/internal/interfaces/middleware/middleware_group"
	"MLC_GO/internal/pkg/logHG"
	UtilsPackage "MLC_GO/internal/pkg/utils"
	HGResponsePakcage "MLC_GO/internal/response"
	"net/http"
)

type HGTestHandler struct {
	Status int64 `json:"status"`
}

// NewTestHandler 创建测试模块 handler。
func NewTestHandler() *HGTestHandler {
	return &HGTestHandler{Status: 0}
}

// TestModuleHandler 注册测试模块路由并装配基础中间件。
func TestModuleHandler() http.Handler {
	handler := NewTestHandler()
	testMux := http.NewServeMux()
	for _, route := range testRouteSpecs(handler) {
		testMux.HandleFunc(route.Path, route.Handler)
	}

	return HGMiddlewarePackage.JSONHeaderMiddleware(
		HGMiddlewarePackage.TIDMiddleware(testMux),
	)
}

// TestRouteCatalog 返回 test 模块完整可调用路径清单。
func TestRouteCatalog(basePrefix string) []HGMiddlewareGroupPackage.HGRouteCatalogItem {
	specs := testRouteSpecs(nil)
	items := make([]HGMiddlewareGroupPackage.HGRouteCatalogItem, 0, len(specs))
	for _, spec := range specs {
		items = append(items, HGMiddlewareGroupPackage.HGRouteCatalogItem{
			Group:    "test",
			Method:   spec.Method,
			Path:     joinRoutePath(basePrefix, spec.Path),
			NeedAuth: spec.NeedAuth,
			Summary:  spec.Summary,
		})
	}

	return items
}

type hgTestRouteSpec struct {
	Method   string
	Path     string
	NeedAuth bool
	Summary  string
	Handler  http.HandlerFunc
}

func testRouteSpecs(handler *HGTestHandler) []hgTestRouteSpec {
	if handler == nil {
		return []hgTestRouteSpec{
			{Method: http.MethodGet, Path: "/ok", NeedAuth: false, Summary: "测试正常返回"},
			{Method: http.MethodGet, Path: "/error", NeedAuth: false, Summary: "测试 panic 恢复链路"},
		}
	}

	return []hgTestRouteSpec{
		{Method: http.MethodGet, Path: "/ok", NeedAuth: false, Summary: "测试正常返回", Handler: handler.OK},
		{Method: http.MethodGet, Path: "/error", NeedAuth: false, Summary: "测试 panic 恢复链路", Handler: handler.Error},
	}
}

func joinRoutePath(prefix string, subPath string) string {
	if prefix == "" {
		return subPath
	}
	if subPath == "" || subPath == "/" {
		return prefix
	}
	if subPath[0] != '/' {
		return prefix + "/" + subPath
	}

	return prefix + subPath
}

// OK 正常返回测试接口。
func (handler *HGTestHandler) OK(w http.ResponseWriter, r *http.Request) {
	tid := UtilsPackage.GetTID(r.Context())
	logHG.DebugFInfo("[TID=%s] 测试 业务逻辑开始", tid)

	resultModel := &HGTestResultModel{User: "Master1💎", Age: 25}
	HGResponsePakcage.WriteJSON(w, r, resultModel)
}

// Error panic 恢复链路测试接口。
func (handler *HGTestHandler) Error(w http.ResponseWriter, r *http.Request) {
	tid := UtilsPackage.GetTID(r.Context())
	logHG.DebugFInfo("[TID=%s] 测试 业务逻辑开始", tid)

	panic("database 连接失败☹️")
}
