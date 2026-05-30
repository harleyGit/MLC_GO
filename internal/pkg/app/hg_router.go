package HGAppPackage

import (
	HGMiddlewarePackage "MLC_GO/internal/pkg/middleware"
	"net/http"
)

// HGRouter 定义路由注册元信息。
type HGRouter struct {
	Method string
	Path   string
	Auth   bool
	Span   string
	Handle http.HandlerFunc
}

// Register 批量注册路由，自动装配基础中间件链。
func Register(mux *http.ServeMux, routes []HGRouter) {
	for _, r := range routes {
		middlewares := []HGMiddlewarePackage.Middleware{
			HGMiddlewarePackage.RecoverMiddleware,
			HGMiddlewarePackage.TraceMiddleware(r.Span),
			HGMiddlewarePackage.AccessLogMiddleware,
			HGMiddlewarePackage.RequestIDMiddleware,
		}
		h := HGMiddlewarePackage.Chain(r.Handle, middlewares...)
		mux.Handle(r.Path, h)
	}
}
