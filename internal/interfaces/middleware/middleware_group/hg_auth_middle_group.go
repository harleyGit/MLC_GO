/*
* @Author: GangHuang harleysor@qq.com
* @Date: 2026-01-26 18:10:07
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-29 20:49:46
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

/* Auth 无关接口【登录、验证码】， 子路由只写相对路径 */
func AuthMiddlewareGoup(userHandler *UserHandlerPackage.UserHandler) http.Handler {

	publicMux := http.NewServeMux()
	publicMux.HandleFunc("/send_code", userHandler.SendCode)
	publicMux.HandleFunc("/register", userHandler.RegisterHandlerV3)
	publicMux.HandleFunc("/login", userHandler.Login)

	recoverHandler := HGMiddlewarePackage.RecoverMiddleware(publicMux)
	loggerHander := HGMiddlewarePackage.LoggerMiddleware(recoverHandler)
	// 先执行（进入时）的是：JSONHeaderMiddleware
	// 后执行（进入时）的是：TIDMiddleware
	// 即：外层中间件先执行进入逻辑，内层中间件后执行进入逻辑。
	// 统一： JOSN + TID【不加Auth】
	// 添加追踪中间件（如 TID）
	withTracing := HGMiddlewarePackage.TIDMiddleware(loggerHander)
	// 添加通用响应头（如 JSON）
	publicHandler := HGMiddlewarePackage.JSONHeaderMiddleware(withTracing)

	return publicHandler
}
