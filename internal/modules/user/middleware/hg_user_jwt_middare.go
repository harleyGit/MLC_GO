/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-22 16:44:22
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-22 17:25:08
 * @FilePath: /MLC_GO/internal/modules/user/middleware/hg_user_jwt_middare.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package UserJWTMiddlewarePackage

import (
	UserServicePackage "MLC_GO/internal/modules/user/service"
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type ctxKey string

const UserIDKey ctxKey = "uid"

/* JWT中间件鉴权 */
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("Authorization")
			if !strings.HasPrefix(h, "Bearer ") {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			token := strings.TrimPrefix(h, "Bearer ")
			claims := &UserServicePackage.HGClaims{}

			t, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
				return UserServicePackage.Secret, nil
			})

			if err != nil || !t.Valid {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		},
	)
}
