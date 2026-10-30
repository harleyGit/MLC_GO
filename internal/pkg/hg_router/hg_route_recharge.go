package HGRouterPackage

import (
	"MLC_GO/internal/modules/recharge/handler"
	hguser "MLC_GO/internal/modules/user/middleware"
	"net/http"
)

const HGRechargeModuleBasePath = "/api/v1/recharge"

// HGNewRechargeRouteGroup 沿用 wallet 的签名守卫和 JWT 链。
func HGNewRechargeRouteGroup(hgHandler *handler.HGHandler) http.Handler {
	return NewRouteGroup(RouteGroupConfig{BasePath: HGRechargeModuleBasePath, AuthMiddleware: hguser.AuthMiddleware}, hgRechargeRoutes(hgHandler))
}

// HGRechargeRouteCatalog 返回独立充值模块完整接口目录。
func HGRechargeRouteCatalog() []RouteCatalogItem {
	return BuildRouteCatalogItems(hgRechargeRoutes(nil))
}

func hgRechargeRoutes(hgHandler *handler.HGHandler) []RouteSpec {
	var hgRecords http.HandlerFunc
	if hgHandler != nil {
		hgRecords = hgHandler.HGRecords
	}
	return []RouteSpec{NewRouteSpec("recharge", http.MethodGet, HGRechargeModuleBasePath, "/records", true, "查询本人充值记录", hgRecords)}
}
