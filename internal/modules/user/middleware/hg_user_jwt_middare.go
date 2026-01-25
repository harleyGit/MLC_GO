/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-22 16:44:22
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-25 19:19:30
 * @FilePath: /MLC_GO/internal/modules/user/middleware/hg_user_jwt_middare.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package UserJWTMiddlewarePackage

import (
	UserServicePackage "MLC_GO/internal/modules/user/service"
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type ctxKey string

const UserIDKey ctxKey = "uid"

/* JWT中间件鉴权 */
func AuthMiddleware(next http.Handler) http.Handler { // 可以从这里传入
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("Authorization")
			if !strings.HasPrefix(h, "Bearer ") {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			token := strings.TrimPrefix(h, "Bearer ")
			claims := &UserServicePackage.HGClaims{}
			parser := jwt.NewParser(
				jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}),
				jwt.WithLeeway(30*time.Second), // ⭐ 非常重要
			)

			t, err := parser.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
				return UserServicePackage.Secret, nil
			})

			// TODO: 🌟🌟🌟多设备登录校验
			// if !repo.Valid(
			// 	r.Context(),
			// 	claims.UserID,
			// 	claims.Device,
			// 	claims.JTI,
			// ) {
			// 	http.Error(w, "token revoked", http.StatusUnauthorized)
			// 	return
			// }

			if err != nil || !t.Valid {
				now := time.Now()
				desc := "invalid token" + err.Error() +
					"\n localTime:" + now.Format("2006-01-02 15:04:05") +
					"\n UTC time:" + now.UTC().Format("2006-01-02 15:04:05")
				http.Error(w, desc, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		},
	)
}
