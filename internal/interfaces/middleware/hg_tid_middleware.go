/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-26 17:57:43
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-27 20:48:58
 * @FilePath: /MLC_GO/internal/interfaces/middleware/hg_tid_middleware.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package HGMiddlewarePackage

import (
	"MLC_GO/internal/pkg/logHG"
	HGResponsePakcage "MLC_GO/internal/response"
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"time"
)

type tidContextKeyType struct{}

var tidContextKey = tidContextKeyType{}

/* 随机生成 tid 【用于追踪请求】 工具函数 */
func generateTID() string {
	return fmt.Sprintf("%016x%016x", rand.Uint64(), rand.Uint64())
}
func generateTIDV1() string {
	return fmt.Sprintf(
		"%d-%06d",
		time.Now().UnixNano(),
		rand.Intn(1000000),
	)
}

func CreateTID() string {
	return generateTIDV1()
}

// 对外暴露安全的读取方法
func GetTID(ctx context.Context) string {
	tid, _ := ctx.Value(tidContextKey).(string)
	return tid
}

/* tid中间件，tid必须放在中间件中。放在http的context可以贯穿生命周期，若是直接放在返回结果的字段里没法全链路追踪了。 */
func TIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tid := generateTID()
		start := time.Now()
		// 放到 context 里供 handler 使用，不要字符串类型 "tid" 做key，应该用 tidContextKey
		// 原因是 字符串容易被其他的覆盖掉， string 无法表达“只属于这个包”；
		// type tidContextKeyType struct{} 1.只存在于这个 package；2.外部包 无法构造同类型的 key
		ctx := context.WithValue(r.Context(), tidContextKey, tid)
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
					tid,
					errModel,
				)
				logHG.DebugFInfo("[TID=%s] <-- %s %s (%v)\n\n", tid, r.Method, r.URL.Path, time.Since(start))

			}
		}()

		next.ServeHTTP(w, r)
		logHG.DebugFInfo("[TID=%s] <-- %s %s (%v)\n\n", tid, r.Method, r.URL.Path, time.Since(start))
	})
}
