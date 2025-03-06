/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-01 21:23:28
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-06 18:01:28
 * @FilePath: /MLC_GO/TestNotes/GenPracticeExample/pkg/util/jwt.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package util

import (
	"MLC_GO/TestNotes/GenPracticeExample/pkg/setting"
	"time"

	"github.com/dgrijalva/jwt-go"
)

var jwtSecret = []byte(setting.JwtSecret)

type Claims struct {
	Username string `json:"username"`
	Password string `json:"password"`
	jwt.StandardClaims //是 jwt-go 提供的标准字段，比如 exp（过期时间）、iat（签发时间）、iss（签发者）等
}

func GenerateToken(username, password string) (string, error) {
	nowTime := time.Now()
	expireTime := nowTime.Add(3 *time.Hour) // 当前时间 + 3小时

	claims := Claims{
		username,
		password,
		/* 
		jwt.StandardClaims 是 github.com/golang-jwt/jwt/v4 包提供的一个标准 JWT 声明（Claims）。
		它用于存储 Token 的基本信息，如：
			过期时间 (ExpiresAt)
			发行者 (Issuer)
			签发时间 (IssuedAt)
			主题 (Subject)
			接收者 (Audience)
			Token ID (Id)

		Issuer: "gin-blog"用途：
			解析 JWT 时，可以检查 Issuer 是否符合预期，防止 Token 被其他系统伪造。
			例如，只接受 Issuer 为 "gin-blog" 的 Token，其他来源的 Token 视为无效。
		*/
		jwt.StandardClaims{
			ExpiresAt: expireTime.Unix(), // 过期时间
			Issuer: "gin-blog", // 发行者 （作用：指定 JWT Token 的发行者（谁创建的这个 Token））可以是：my-app, example.com
		},
	}

	// NewWithClaims(method SigningMethod, claims Claims)，method对应着SigningMethodHMAC struct{}，其包含SigningMethodHS256、SigningMethodHS384、SigningMethodHS512三种crypto.Hash方案
	// 使用 HS256 算法
	tokenClaims := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	// func (t *Token) SignedString(key interface{}) 该方法内部生成签名字符串，再用于获取完整、已签名的token
	token, err := tokenClaims.SignedString(jwtSecret) // 使用密钥签名

	return token, err
}

func ParseToken(token string) (*Claims, error) {
	// func (p *Parser) ParseWithClaims 用于解析鉴权的声明，方法内部主要是具体的解码和校验的过程，最终返回*Token
	tokenClaims, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		return jwtSecret, nil // 返回密钥
	})

	// 检查 Token 是否有效
	if tokenClaims != nil {
		// func (m MapClaims) Valid() 验证基于时间的声明exp, iat, nbf，注意如果没有任何声明在令牌中，仍然会被认为是有效的。并且对于时区偏差没有计算方法
		if claims, ok := tokenClaims.Claims.(*Claims); ok && tokenClaims.Valid {
			return claims, nil
		}
	}

	return nil, err
}