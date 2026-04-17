/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-26 17:57:43
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-29 17:43:51
 * @FilePath: /MLC_GO/internal/interfaces/middleware/hg_tid_middleware.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package HGMiddlewarePackage

import (
	"MLC_GO/internal/pkg/logHG"
	UtilsPackage "MLC_GO/internal/pkg/utils"
	"net/http"
	"time"
)

// RequestTIDInterceptor 注入请求链路追踪 ID，并输出请求级耗时日志。
func RequestTIDInterceptor(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tid := UtilsPackage.GenerateTID()
		start := time.Now()
		ctx := UtilsPackage.InjectTID(r.Context(), tid)
		r = r.WithContext(ctx)

		logHG.DebugFInfo("[TID=%s] --> %s %s \n", tid, r.Method, r.URL.Path)
		// 无论正常返回还是 panic，都会输出结束日志；panic 交给外层 RecoverInterceptor 统一恢复。
		defer func() {
			logHG.DebugFInfo("[TID=%s] <-- %s %s (%v)\n\n", tid, r.Method, r.URL.Path, time.Since(start))
		}()

		next.ServeHTTP(w, r)
	})
}

// TIDMiddleware 兼容旧方法名，内部转发到拦截器实现。
func TIDMiddleware(next http.Handler) http.Handler {
	return RequestTIDInterceptor(next)
}
