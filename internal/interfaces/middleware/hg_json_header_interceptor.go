/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-26 17:33:01
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-27 11:36:29
 * @FilePath: /MLC_GO/internal/interfaces/middleware/hg_header_middleware.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package HGMiddlewarePackage

import "net/http"

// JSONHeaderInterceptor 统一设置 JSON 响应头，保证客户端按 JSON 解析。
func JSONHeaderInterceptor(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// 可以不加吗？
		// 技术上可以不加，浏览器和前端依然能收到响应内容，但不推荐：
		//     ◦    前端 fetch 或 axios 默认会根据 Content-Type 解析响应（比如自动 parse JSON）
		//     ◦    不加，前端可能需要手动 JSON.parse(res)
		//     ◦    加上之后前端会自动识别 JSON，更安全和标准
		// ✅ 推荐加上，尤其是返回 JSON 的接口。
		// 设置 Content-Type
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		next.ServeHTTP(w, r)
	})
}

// JSONHeaderMiddleware 兼容旧方法名，内部转发到拦截器实现。
func JSONHeaderMiddleware(next http.Handler) http.Handler {
	return JSONHeaderInterceptor(next)
}
