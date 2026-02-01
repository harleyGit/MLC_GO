/*
* @Author: GangHuang harleysor@qq.com
* @Date: 2026-01-26 19:48:25
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-02-01 17:07:03

* @FilePath: /MLC_GO/internal/interfaces/middleware/middleware_group/hg_user_middle_group.go
* @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE

* 需要用户登录的接口
*/
package HGMiddlewareGroupPackage

import (
	HGMiddlewarePackage "MLC_GO/internal/interfaces/middleware"
	UserHandlerPackage "MLC_GO/internal/modules/user/handler"
	UserJWTMiddlewarePackage "MLC_GO/internal/modules/user/middleware"
	HGServerPackage "MLC_GO/server"
	"net/http"
)

/* Auth 无关接口【登录、验证码】 */
func UserMiddlewareGoup(userHandler *UserHandlerPackage.UserHandler) http.Handler {

	userMux := http.NewServeMux()
	userMux.HandleFunc("/info", userHandler.Profile)
	// userMux.HandleFunc("/user/logout", userHandler.Logout)//登出

	guard := HGMiddlewarePackage.NewAPIGuard(
		HGServerPackage.UserMethodRules(),
	)
	// 统一： JOSN + TID + Auth
	authHandler := UserJWTMiddlewarePackage.AuthMiddleware(userMux)
	tidHandler :=  HGMiddlewarePackage.TIDMiddleware(authHandler)
	jsonHandler := HGMiddlewarePackage.JSONHeaderMiddleware(tidHandler)

	return guard.MethodGuardMiddlewareV3(jsonHandler)
}