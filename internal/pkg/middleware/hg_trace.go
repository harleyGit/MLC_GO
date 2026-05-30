package HGMiddlewarePackage

import (
	HGTracePackage "MLC_GO/internal/pkg/trace"
	UtilsPackage "MLC_GO/internal/pkg/utils"
	"net/http"
)

// TraceMiddleware 创建 span 追踪上下文并贯穿请求生命周期。
func TraceMiddleware(spanName string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tid := UtilsPackage.GetTID(r.Context())
			ctx := HGTracePackage.NewTrace(r.Context(), spanName, tid)
			tc := HGTracePackage.Get(ctx)

			defer HGTracePackage.End(tc)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// TraceInterceptor 兼容旧方法名。
func TraceInterceptor(spanName string) Middleware {
	return TraceMiddleware(spanName)
}
