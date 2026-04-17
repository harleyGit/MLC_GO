/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-29 19:52:08
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-29 19:55:18
 * @FilePath: /MLC_GO/internal/interfaces/middleware/hg_log_middle.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package HGMiddlewarePackage

import (
	"MLC_GO/internal/pkg/logHG"
	"net/http"
	"time"
)

// AccessLogInterceptor 记录请求方法、路径和耗时，作为基础访问日志拦截器。
func AccessLogInterceptor(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)

		logHG.DebugFInfo(
			`{"method":"%s", "path":"%s", "cost_ms":%d}`,
			r.Method,
			r.URL.Path,
			time.Since(start).Milliseconds(),
		)
	})
}

// LoggerMiddleware 兼容旧方法名，内部转发到拦截器实现。
func LoggerMiddleware(next http.Handler) http.Handler {
	return AccessLogInterceptor(next)
}
