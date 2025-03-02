/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-01 21:37:55
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-02 12:33:31
 * @FilePath: /MLC_GO/TestNotes/PracticeGenExample/middleware/jwt/jwt.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package jwt

import (
	"MLC_GO/TestNotes/PracticeGenExample/pkg/e"
	"MLC_GO/TestNotes/PracticeGenExample/pkg/util"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

/*JWT() 返回的是一个匿名函数，这个匿名函数就是实际的 Gin 中间件，会在请求进入时执行

	gin.HandlerFunc 是 func(ctx *gin.Context) 形式的函数，用于处理 HTTP 请求。
 */
func JWT() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var code int
		var data interface{}

		code = e.SUCCESS
		token := ctx.Query("token")
		if token == "" {
			code = e.INVALID_PARAMS
		} else {
			claims, err := util.ParseToken(token)
			if err != nil {
				code = e.ERROR_AUTH_CHECK_TOKEN_FAIL
			} else if time.Now().Unix() > claims.ExpiresAt {
				code = e.ERROR_AUTH_CHECK_TOKEN_TIMEOUT
			}
		}

		if code != e.SUCCESS {
			ctx.JSON(http.StatusUnauthorized, gin.H{
				"code" : code,
				"msg": e.GetMsg(code),
				"data": data,
			})
			/* 
			ctx.Abort() 立即终止当前请求，并且不会执行后续的 handler 或中间件。
			这个方法常用于拦截非法请求（比如 JWT 认证失败）。
			*/
			ctx.Abort() // 终止请求的后续处理
			return
		}
		// ctx.Next() 用于执行后续的中间件或 handler
		ctx.Next() // 继续执行下一个中间件（无效，因为 Abort 了）
	}
}