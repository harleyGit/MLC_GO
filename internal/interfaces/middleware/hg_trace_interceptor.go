/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-28 10:40:35
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-28 20:20:57
 * @FilePath: /MLC_GO/internal/interfaces/middleware/hg_trace_middleware.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package HGMiddlewarePackage

import (
	HGTracePackage "MLC_GO/internal/pkg/trace"
	UtilsPackage "MLC_GO/internal/pkg/utils"
	"net/http"
)

// TraceInterceptor 创建 span 追踪上下文并贯穿请求生命周期。
func TraceInterceptor(spanName string) HGHTTPInterceptor {
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

// TraceMiddleware 兼容旧方法名，内部转发到拦截器实现。
func TraceMiddleware(spanName string) func(http.Handler) http.Handler {
	return TraceInterceptor(spanName)
}
