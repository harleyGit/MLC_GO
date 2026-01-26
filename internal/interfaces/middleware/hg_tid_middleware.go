/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-26 17:57:43
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-26 18:03:02
 * @FilePath: /MLC_GO/internal/interfaces/middleware/hg_tid_middleware.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package HGMiddlewarePackage

import (
	"MLC_GO/internal/pkg/logHG"
	"context"
	"fmt"
	"math/rand"
	"net/http"
)

/* 随机生成 tid 【用于追踪请求】 工具函数 */
func generateTID() string {
	return  fmt.Sprintf("%016x%016x", rand.Uint64(), rand.Uint64())
}

func TIDMiddleware(next http.Handler) http.Handler {
	return  http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tid := generateTID()
		// 放到 context 里供 handler 使用
		ctx := context.WithValue(r.Context(), "tid", tid)
		r = r.WithContext(ctx)

		logHG.DebugFInfo("tid=%s 请求开始 %s %s\n", tid, r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
		logHG.DebugFInfo("tid=%s 请求结束 %s %s\n", tid, r.Method, r.URL.Path)
	})
}