/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-26 17:33:01
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-26 17:33:12
 * @FilePath: /MLC_GO/internal/interfaces/middleware/hg_header_middleware.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package HGMiddlewarePackage

import "net/http"

func JSONHeaderMiddleware(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		next.ServeHTTP(w, r)
	})
}
