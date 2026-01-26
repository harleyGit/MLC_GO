/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-26 19:59:39
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-26 20:04:03
 * @FilePath: /MLC_GO/internal/interfaces/middleware/middleware_group/hg_order_middle_group.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 
 * 订单/购买模块（以后扩展使用）
 */
package HGMiddlewareGroupPackage

/*

import (
	HGMiddlewarePackage "MLC_GO/internal/interfaces/middleware"
	UserJWTMiddlewarePackage "MLC_GO/internal/modules/user/middleware"
	"net/http"
)


func OrderMiddlewareGroup(orderHandler interface{}) http.Handler {

	orderMux := http.NewServeMux()
	orderMux.HandleFunc("/order/create", orderHandler.Create)
	orderMux.HandleFunc("/order/List", orderHandler.List)

	OrderHandlerWithAuth := HGMiddlewarePackage.JSONHeaderMiddleware(
		HGMiddlewarePackage.TIDMiddleware(
			UserJWTMiddlewarePackage.AuthMiddleware(orderMux),
		),
	)

	return OrderHandlerWithAuth
}
*/	