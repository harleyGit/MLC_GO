/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-13 11:16:39
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-14 20:03:43
 * @FilePath: /MLC_GO/internal/pkg/middleware/hg_auth.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package PkgMiddlewarePackage

import (
	PersistenceRedisPackage "MLC_GO/internal/infrastructure/persistence/redis"
	PresentersPackage "MLC_GO/internal/interfaces/presenters"
	"net/http"
)

/* Token校验中间件 */
func TokenAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if token == "" {
			PresentersPackage.WriteJSON(w, map[string]string{"error": "缺少Token"})
			return
		}

		_, err := PersistenceRedisPackage.RDB.Get(r.Context(), "token:"+token).Result()
		if err != nil {
			PresentersPackage.WriteJSON(w, map[string]string{"error": "无效Token"})
			return
		}
		next(w, r)
	}
}