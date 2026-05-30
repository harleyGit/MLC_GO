package HGTestHandlerPackage

import (
	HGMiddlewarePackage "MLC_GO/internal/pkg/middleware"
	HGMiddlewareGroupPackage "MLC_GO/internal/pkg/hg_router"
	"MLC_GO/internal/pkg/logHG"
	UtilsPackage "MLC_GO/internal/pkg/utils"
	HGResponsePakcage "MLC_GO/internal/response"
	"net/http"
)

type HGTestHandler struct {
	Status int64 `json:"status"`
}

const (
	// TestModuleBasePath 是 test 模块对外暴露的统一 API 前缀。
	TestModuleBasePath = "/api/v1/test"
)

// NewTestHandler 创建测试模块 handler。
func NewTestHandler() *HGTestHandler {
	return &HGTestHandler{Status: 0}
}

// TestModuleHandler 注册测试模块路由并装配基础中间件。
func TestModuleHandler() http.Handler {
	handler := NewTestHandler()
	testMux := http.NewServeMux()
	for _, route := range testRouteSpecs(handler) {
		testMux.HandleFunc(route.SubPath, route.Handler)
	}

	return HGMiddlewarePackage.ChainInterceptors(
		testMux,
		HGMiddlewarePackage.RecoverInterceptor,
		HGMiddlewarePackage.AccessLogInterceptor,
		HGMiddlewarePackage.RequestTIDInterceptor,
		HGMiddlewarePackage.JSONHeaderInterceptor,
	)
}

// TestRouteCatalog 返回 test 模块完整可调用路径清单。
func TestRouteCatalog() []HGMiddlewareGroupPackage.HGRouteCatalogItem {
	specs := testRouteSpecs(nil)
	items := make([]HGMiddlewareGroupPackage.HGRouteCatalogItem, 0, len(specs))
	for _, spec := range specs {
		items = append(items, HGMiddlewareGroupPackage.HGRouteCatalogItem{
			Group:    spec.Group,
			Method:   spec.Method,
			Path:     spec.FullPath,
			NeedAuth: spec.NeedAuth,
			Summary:  spec.Summary,
		})
	}

	return items
}

type hgTestRouteSpec struct {
	Group    string
	Method   string
	SubPath  string
	FullPath string
	NeedAuth bool
	Summary  string
	Handler  http.HandlerFunc
}

func newTestRouteSpec(method string, subPath string, needAuth bool, summary string, handler http.HandlerFunc) hgTestRouteSpec {
	return hgTestRouteSpec{
		Group:    "test",
		Method:   method,
		SubPath:  subPath,
		FullPath: joinRoutePath(TestModuleBasePath, subPath),
		NeedAuth: needAuth,
		Summary:  summary,
		Handler:  handler,
	}
}

func testRouteSpecs(handler *HGTestHandler) []hgTestRouteSpec {
	if handler == nil {
		return []hgTestRouteSpec{
			newTestRouteSpec(http.MethodGet, "/ok", false, "测试正常返回", nil),
			newTestRouteSpec(http.MethodGet, "/error", false, "测试 panic 恢复链路", nil),
		}
	}

	return []hgTestRouteSpec{
		newTestRouteSpec(http.MethodGet, "/ok", false, "测试正常返回", handler.OK),
		newTestRouteSpec(http.MethodGet, "/error", false, "测试 panic 恢复链路", handler.Error),
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
