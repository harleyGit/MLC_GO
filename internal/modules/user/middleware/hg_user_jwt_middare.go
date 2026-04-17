/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-22 16:44:22
 * @LastEditors: Harley harelysoa@qq.com
 * @LastEditTime: 2026-04-18 03:02:29
 * @FilePath: /MLC_GO/internal/modules/user/middleware/hg_user_jwt_middare.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package UserJWTMiddlewarePackage

import (
	UserServicePackage "MLC_GO/internal/modules/user/service"
	PkGDevicePackage "MLC_GO/internal/pkg/device"
	HGResponsePakcage "MLC_GO/internal/response"
	"context"
	"errors"
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
				w.WriteHeader(http.StatusUnauthorized)
				HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.TokenInvalidCode, HGResponsePakcage.TokenInvalidFailDesc)
				return
			}

			// strings.TrimPrefix(s, prefix string) 会检查 s 是否以 prefix 开头
			// 如果是，就返回去掉前缀后的字符串
			// 如果不是，就返回原字符串不变
			token := strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
			if token == "" {
				w.WriteHeader(http.StatusUnauthorized)
				HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.TokenInvalidCode, HGResponsePakcage.TokenInvalidFailDesc)
				return
			}
			claims := &UserServicePackage.HGClaims{}
			parser := jwt.NewParser(
				jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}),
				jwt.WithLeeway(30*time.Second), // ⭐ 非常重要
			)

			// 解析 JWT（JSON Web Token）并验证其签名 的函数。它接受一个 JWT 字符串、一个用于存储解析结果的 claims 结构体，以及一个回调函数来提供验证签名所需的密钥。
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
				desc := "invalid token"
				if err != nil {
					desc += ": " + err.Error()
				}
				desc +=
					"\n localTime:" + now.Format("2006-01-02 15:04:05") +
						"\n UTC time:" + now.UTC().Format("2006-01-02 15:04:05")

				w.WriteHeader(http.StatusUnauthorized)
				HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.TokenInvalidCode, HGResponsePakcage.TokenInvalidFailDesc + desc)
				return
			}
			if err := validateAccessClaims(r, claims); err != nil {
				w.WriteHeader(http.StatusUnauthorized)
				HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.TokenInvalidCode, HGResponsePakcage.TokenInvalidFailDesc)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		},
	)
}

// validateAccessClaims 统一校验 access token 的关键 claims，避免无效 token 进入业务层。
func validateAccessClaims(r *http.Request, claims *UserServicePackage.HGClaims) error {
	if claims == nil {
		return errors.New("nil claims")
	}
	// 验证 token 类型,如果 token type 不为空且不是 "access"，就拒绝
	if claims.TokenTp != "" && claims.TokenTp != "access" {
		return errors.New("token type invalid")
	}
	// 验证发行者,确保 token 是由 mlc-go 系统生成，防止第三方伪造
	if claims.Issuer != "" && claims.Issuer != "mlc-go" {
		return errors.New("token issuer invalid")
	}
	// 验证主题（Subject）,必须是 "user-token"，保证 token 用于用户认证。
	if claims.Subject != "" && claims.Subject != "user-token" {
		return errors.New("token subject invalid")
	}
	// 登录时会把设备指纹放入 JWT，这里顺带校验当前请求是否来自同一设备环境。
	// PkGDevicePackage.Fingerprint(r) → 当前请求的设备指纹
	if claims.Device != "" && claims.Device != PkGDevicePackage.Fingerprint(r) {
		return errors.New("device fingerprint mismatch")
	}

	return nil
}
