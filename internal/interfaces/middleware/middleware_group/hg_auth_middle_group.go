/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-26 18:10:07
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-26 19:58:55
 * @FilePath: /MLC_GO/internal/interfaces/middleware/middleware_group/hg_tid_middle_group.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 
 * Auth 无关接口
 */
package HGMiddlewareGroupPackage

import (
	HGMiddlewarePackage "MLC_GO/internal/interfaces/middleware"
	UserHandlerPackage "MLC_GO/internal/modules/user/handler"
	"net/http"
)

/* Auth 无关接口【登录、验证码】 */
func AuthMiddlewareGoup(userHandler *UserHandlerPackage.UserHandler) http.Handler {

	publicMux := http.NewServeMux()
	publicMux.HandleFunc("/auth/send_code", userHandler.SendCode)
	publicMux.HandleFunc("/user/register", userHandler.RegisterHandlerV3)
	publicMux.HandleFunc("/auth/login", userHandler.Login)

	// 统一： JOSN + TID【不加Auth】
	publicHandler := HGMiddlewarePackage.JSONHeaderMiddleware(
		HGMiddlewarePackage.TIDMiddleware(publicMux),
	)

	return publicHandler
}
