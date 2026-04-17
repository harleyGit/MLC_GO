/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-13 11:16:39
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-29 17:36:48
 * @FilePath: /MLC_GO/internal/pkg/middleware/hg_auth.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 * 生产级中间件
 */
package PkgMiddlewarePackage

import (
	PersistenceRedisPackage "MLC_GO/internal/infrastructure/persistence/redis"
	HGResponsePakcage "MLC_GO/internal/response"
	"net/http"
)

func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if token == "" {
			w.WriteHeader(http.StatusUnauthorized)
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.TokenInvalidCode, HGResponsePakcage.TokenInvalidFailDesc)
			return
		}
		if _, err := PersistenceRedisPackage.GetFromRedis(nil, "token:"+token); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.TokenInvalidCode, HGResponsePakcage.TokenInvalidFailDesc)
			return
		}
		next(w, r)
	}
}

/* Token校验中间件 */
func TokenAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if token == "" {
			w.WriteHeader(http.StatusUnauthorized)
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.TokenInvalidCode, HGResponsePakcage.TokenInvalidFailDesc)
			return
		}

		_, err := PersistenceRedisPackage.RDB.Get(r.Context(), "token:"+token).Result()
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.TokenInvalidCode, HGResponsePakcage.TokenInvalidFailDesc)
			return
		}
		next(w, r)
	}
}
