package HGRouterPackage

import (
	"MLC_GO/internal/modules/product"
	"MLC_GO/internal/modules/user/middleware"
	"MLC_GO/internal/pkg/server"
	"net/http"
)

const HGMallModuleBasePath = "/api/v1/mall"

// HGNewMallRouteGroup 复用现有签名、Guard 和 JWT 链；审核权限由 product.Service 再次查库。
func HGNewMallRouteGroup(h *product.HGHandler) http.Handler {
	return NewRouteGroup(RouteGroupConfig{BasePath: HGMallModuleBasePath, Rules: HGServerPackage.HGMallMethodRules(), AuthMiddleware: middleware.AuthMiddleware}, hgMallRoutes(h))
}

func HGMallRouteCatalog() []RouteCatalogItem { return BuildRouteCatalogItems(hgMallRoutes(nil)) }

func hgMallRoutes(h *product.HGHandler) []RouteSpec {
	var list, detail, mine, publish, merchant, apply, pendingMerchants, reviewMerchant, pendingProducts, reviewProduct http.HandlerFunc
	if h != nil {
		list, detail, mine, publish, merchant, apply = h.HGList, h.HGDetail, h.HGMine, h.HGPublish, h.HGMerchant, h.HGApply
		pendingMerchants, reviewMerchant, pendingProducts, reviewProduct = h.HGPendingMerchants, h.HGReviewMerchant, h.HGPendingProducts, h.HGReviewProduct
	}
	return []RouteSpec{
		NewRouteSpec("mall", http.MethodGet, HGMallModuleBasePath, "/products", true, "商品目录", list),
		NewRouteSpec("mall", http.MethodGet, HGMallModuleBasePath, "/products/detail", true, "商品详情", detail),
		NewRouteSpec("mall", http.MethodGet, HGMallModuleBasePath, "/products/mine", true, "商家本人商品", mine),
		NewRouteSpec("mall", http.MethodPost, HGMallModuleBasePath, "/products/publish", true, "发布单 SKU 商品草稿", publish),
		NewRouteSpec("mall", http.MethodGet, HGMallModuleBasePath, "/merchant", true, "本人商家入驻状态", merchant),
		NewRouteSpec("mall", http.MethodPost, HGMallModuleBasePath, "/merchant/apply", true, "申请商家入驻", apply),
		NewRouteSpec("mall", http.MethodGet, HGMallModuleBasePath, "/review/merchants", true, "待审核商家", pendingMerchants),
		NewRouteSpec("mall", http.MethodPost, HGMallModuleBasePath, "/review/merchants/decision", true, "审核商家", reviewMerchant),
		NewRouteSpec("mall", http.MethodGet, HGMallModuleBasePath, "/review/products", true, "待审核商品", pendingProducts),
		NewRouteSpec("mall", http.MethodPost, HGMallModuleBasePath, "/review/products/decision", true, "审核商品", reviewProduct),
	}
}
