package HGRouterPackage

import (
	hguser "MLC_GO/internal/modules/user/middleware"
	"MLC_GO/internal/modules/wallet/handler"
	hgserver "MLC_GO/internal/pkg/server"
	"net/http"
)

const HGWalletModuleBasePath = "/api/v1/wallet"

// HGNewWalletRouteGroup 沿用签名守卫和JWT链，不开放匿名手机页面接口。
func HGNewWalletRouteGroup(hgHandler *handler.HGHandler) http.Handler {
	return NewRouteGroup(RouteGroupConfig{BasePath: HGWalletModuleBasePath, Rules: hgserver.HGWalletMethodRules(), AuthMiddleware: hguser.AuthMiddleware}, hgWalletRoutes(hgHandler))
}

// HGWalletRouteCatalog 返回钱包完整用户接口目录。
func HGWalletRouteCatalog() []RouteCatalogItem { return BuildRouteCatalogItems(hgWalletRoutes(nil)) }

func hgWalletRoutes(hgHandler *handler.HGHandler) []RouteSpec {
	var hgBalance, hgSKUs, hgCreate, hgDetail, hgPay http.HandlerFunc
	if hgHandler != nil {
		hgBalance, hgSKUs, hgCreate, hgDetail, hgPay = hgHandler.HGBalance, hgHandler.HGSKUs, hgHandler.HGCreateOrder, hgHandler.HGOrderDetail, hgHandler.HGPay
	}
	return []RouteSpec{
		NewRouteSpec("wallet", http.MethodGet, HGWalletModuleBasePath, "/balance", true, "查询当前用户平台币余额", hgBalance),
		NewRouteSpec("wallet", http.MethodGet, HGWalletModuleBasePath, "/recharge/skus", true, "查询有效充值档位", hgSKUs),
		NewRouteSpec("wallet", http.MethodPost, HGWalletModuleBasePath, "/recharge/orders", true, "创建十分钟充值订单", hgCreate),
		NewRouteSpec("wallet", http.MethodGet, HGWalletModuleBasePath, "/recharge/orders/detail", true, "查询本人充值订单", hgDetail),
		NewRouteSpec("wallet", http.MethodPost, HGWalletModuleBasePath, "/recharge/orders/pay", true, "付款入口，渠道尚未配置", hgPay),
	}
}
