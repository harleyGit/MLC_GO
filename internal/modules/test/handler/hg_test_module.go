package HGTestHandlerPackage

import (
	HGMiddlewarePackage "MLC_GO/internal/pkg/middleware"
	"net/http"

	HGHandlerPackage "MLC_GO/internal/handler"
)

// TestModule 实现 Module 接口，测试路由。
type TestModule struct {
	handler *HGTestHandler
}

// NewTestModule 创建测试模块实例。
func NewTestModule() *TestModule {
	return &TestModule{handler: NewTestHandler()}
}

func (m *TestModule) Name() string {
	return "test"
}

func (m *TestModule) BasePath() string {
	return TestModuleBasePath
}

func (m *TestModule) Handler() http.Handler {
	testMux := http.NewServeMux()
	for _, route := range testRouteSpecs(m.handler) {
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

// RegisterModules 注册测试模块。
func RegisterModules() {
	HGHandlerPackage.RegisterModule(NewTestModule())
}
