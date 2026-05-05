package HGMiddlewareGroupPackage

/*
 * 订单/购买模块（以后扩展使用）
 *
 * 示例代码：
 *
 * import (
 *     HGMiddlewarePackage "MLC_GO/internal/interfaces/middleware"
 *     UserJWTMiddlewarePackage "MLC_GO/internal/modules/user/middleware"
 *     "net/http"
 * )
 *
 * const OrderModuleBasePath = "/api/v1/order"
 *
 * func NewOrderRouteGroup(orderHandler *OrderHandler) http.Handler {
 *     specs := []RouteSpec{
 *         NewRouteSpec("order", http.MethodPost, OrderModuleBasePath, "/create", true, "创建订单", orderHandler.Create),
 *         NewRouteSpec("order", http.MethodGet, OrderModuleBasePath, "/list", true, "订单列表", orderHandler.List),
 *     }
 *
 *     orderMux := http.NewServeMux()
 *     BindRouteSpecs(orderMux, specs)
 *
 *     protected := HGMiddlewarePackage.Chain(
 *         orderMux,
 *         UserJWTMiddlewarePackage.AuthMiddleware,
 *     )
 *
 *     return HGMiddlewarePackage.Chain(
 *         protected,
 *         HGMiddlewarePackage.RequestIDMiddleware,
 *         HGMiddlewarePackage.AccessLogMiddleware,
 *         HGMiddlewarePackage.RecoverMiddleware,
 *         HGMiddlewarePackage.JSONHeaderMiddleware,
 *     )
 * }
 */
