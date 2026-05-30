/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-05-30 08:42:39
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-05-30 08:42:41
 * @FilePath: /MLC_GO/internal/pkg/hg_context/hg_user.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package hgcontext

import (
	UserJWTMiddlewarePackage "MLC_GO/internal/modules/user/middleware"
	UserServicePackage "MLC_GO/internal/modules/user/service"
	"net/http"
	"strings"
)

// CurrentUserID 从 JWT 中间件写入的 context 中提取当前登录用户。
// 视频投稿必须绑定用户，handler 层不允许使用前端传入 user_id，避免越权写入。
func CurrentUserID(r *http.Request) (string, bool) {
	/*
				ctx := r.Context()
				r = r.WithContext(ctx)

				// 根据 Key 获取 Value， 这个Key通常在 JWT 中间件里会提前存进去，类似于如下：
				claims := &HGClaims{
					UserID: 10001,
					Username: "admin",
				}
				ctx := context.WithValue(
				    r.Context(),
				    UserIDKey,
				    claims,
				)
				value := ctx.Value(UserJWTMiddlewarePackage.UserIDKey)

				// 取出来就是这样：
				&HGClaims{
		    	UserID: 10001,
		    	Username: "admin",
				}

				// 类型断言，返回来的是interface{}，若是要断言
				claims, ok := value.(*UserServicePackage.HGClaims)
	*/
	claims, ok := r.Context().Value(UserJWTMiddlewarePackage.UserIDKey).(*UserServicePackage.HGClaims)
	if !ok || claims == nil || strings.TrimSpace(claims.UserID) == "" {
		return "", false
	}
	return strings.TrimSpace(claims.UserID), true
}
