/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-29 19:56:24
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-29 20:22:38
 * @FilePath: /MLC_GO/internal/interfaces/middleware/hg_recover.middle.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AEf
 */
package HGMiddlewarePackage

import (
	HGResponsePakcage "MLC_GO/internal/response"
	"net/http"
)

// RecoverInterceptor 统一拦截 panic，防止请求链路异常中断。
func RecoverInterceptor(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				HGResponsePakcage.FailResult[any](w, r,
					HGResponsePakcage.InternalErrorCode,
					"panic",
				)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// RecoverMiddleware 兼容旧方法名，内部转发到拦截器实现。
func RecoverMiddleware(next http.Handler) http.Handler {
	return RecoverInterceptor(next)
}
