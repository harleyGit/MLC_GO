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
	HGResponsePakcage "MLC_GO/internal/response"
	"net/http"
	"time"
)

/* tid中间件，tid必须放在中间件中。放在http的context可以贯穿生命周期，若是直接放在返回结果的字段里没法全链路追踪了。 */
func TIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tid := UtilsPackage.GenerateTID()
		start := time.Now()
		ctx := UtilsPackage.InjectTID(r.Context(), tid)
		r = r.WithContext(ctx)

		logHG.DebugFInfo("[TID=%s] --> %s %s \n", tid, r.Method, r.URL.Path)
		// 捕获 panic
		defer func() {
			if err := recover(); err != nil {
				logHG.DebugFInfo("[TID=%s] 🧨 PANIC: %v \n", tid, err)
				errModel := HGResponsePakcage.HGErrorResult{
					Code:    http.StatusInternalServerError,
					Message: "internal server error",
				}
				HGResponsePakcage.WriteJSON(
					w,
					r,
					errModel,
				)
				logHG.DebugFInfo("[TID=%s] <-- %s %s (%v)\n\n", tid, r.Method, r.URL.Path, time.Since(start))

			}
		}()

		next.ServeHTTP(w, r)
		logHG.DebugFInfo("[TID=%s] <-- %s %s (%v)\n\n", tid, r.Method, r.URL.Path, time.Since(start))
	})
}
