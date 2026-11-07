package HGRouterPackage

import (
	"MLC_GO/internal/modules/mallcny"
	"MLC_GO/internal/modules/product"
	hguser "MLC_GO/internal/modules/user/middleware"
	HGPermissionPackage "MLC_GO/internal/pkg/permission"
	"net/http"
)

const HGMallModuleBasePath = "/api/v1/mall"

// HGNewMallRouteGroup 复用现有签名、Guard 和 JWT 链；审核权限由 product.Service 再次查库。
func HGNewMallRouteGroup(h *product.HGHandler, hgAuthorizers ...HGRouteAuthorizer) http.Handler {
	return HGNewMallRouteGroupWithCNY(h, nil, hgAuthorizers...)
}

// HGNewMallRouteGroupWithCNY 在原商城 JWT、签名和 Guard 下追加隔离 CNY 路由，不改动 M币接口。
func HGNewMallRouteGroupWithCNY(h *product.HGHandler, cny *mallcny.HGHandler, hgAuthorizers ...HGRouteAuthorizer) http.Handler {
	specs := append(hgMallRoutes(h), hgCNYRoutes(cny)...)
	group := NewRouteGroup(RouteGroupConfig{BasePath: HGMallModuleBasePath, Authorizer: hgFirstAuthorizer(hgAuthorizers), AuthMiddleware: hguser.AuthMiddleware}, specs)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 订单地址、手机号与商家申请均属于登录态数据，不允许代理或浏览器缓存。
		w.Header().Set("Cache-Control", "no-store")
		group.ServeHTTP(w, r)
	})
}

func HGMallRouteCatalog() []RouteCatalogItem {
	return BuildRouteCatalogItems(append(hgMallRoutes(nil), hgCNYRoutes(nil)...))
}

func hgCNYRoutes(h *mallcny.HGHandler) []RouteSpec {
	var list, detail, publish, createCoupon, coupons, claim, preview, buy, groups, group, order, orders, pay, refund, cancel, capabilities http.HandlerFunc
	if h != nil {
		list, detail, publish = h.HGList, h.HGDetail, h.HGPublish
		createCoupon, coupons, claim = h.HGCreateCoupon, h.HGCoupons, h.HGClaimCoupon
		preview, buy = h.HGPreview, h.HGBuy
		groups, group = h.HGGroups, h.HGGroup
		order, orders, pay, refund, cancel, capabilities = h.HGOrder, h.HGOrders, h.HGDebugPay, h.HGDebugRefund, h.HGCancel, h.HGCapabilities
	}
	return []RouteSpec{
		NewRouteSpec("mall_cny", http.MethodGet, HGMallModuleBasePath, "/cny/products", true, "人民币商品目录", list),
		NewRouteSpec("mall_cny", http.MethodGet, HGMallModuleBasePath, "/cny/products/detail", true, "人民币商品详情", detail),
		NewRouteSpec("mall_cny", http.MethodPost, HGMallModuleBasePath, "/cny/products/publish", true, "沙箱商家发布人民币商品", publish),
		NewRouteSpec("mall_cny", http.MethodPost, HGMallModuleBasePath, "/cny/coupons/create", true, "沙箱商家创建优惠券", createCoupon),
		NewRouteSpec("mall_cny", http.MethodGet, HGMallModuleBasePath, "/cny/coupons", true, "人民币商品优惠券", coupons),
		NewRouteSpec("mall_cny", http.MethodPost, HGMallModuleBasePath, "/cny/coupons/claim", true, "领取商品优惠券", claim),
		NewRouteSpec("mall_cny", http.MethodPost, HGMallModuleBasePath, "/cny/orders/preview", true, "服务端试算人民币商品", preview),
		NewRouteSpec("mall_cny", http.MethodPost, HGMallModuleBasePath, "/cny/orders", true, "幂等预占库存和下单", buy),
		NewRouteSpec("mall_cny", http.MethodGet, HGMallModuleBasePath, "/cny/groups", true, "有界可加入拼团", groups),
		NewRouteSpec("mall_cny", http.MethodGet, HGMallModuleBasePath, "/cny/groups/detail", true, "拼团进度", group),
		NewRouteSpec("mall_cny", http.MethodGet, HGMallModuleBasePath, "/cny/orders/detail", true, "本人订单", order),
		NewRouteSpec("mall_cny", http.MethodGet, HGMallModuleBasePath, "/cny/orders", true, "本人有界订单列表", orders),
		NewRouteSpec("mall_cny", http.MethodPost, HGMallModuleBasePath, "/cny/orders/pay", true, "隔离环境模拟支付宝付款", pay),
		NewRouteSpec("mall_cny", http.MethodPost, HGMallModuleBasePath, "/cny/orders/refund", true, "隔离环境模拟退款", refund),
		NewRouteSpec("mall_cny", http.MethodPost, HGMallModuleBasePath, "/cny/orders/cancel", true, "取消待付款订单", cancel),
		NewRouteSpec("mall_cny", http.MethodGet, HGMallModuleBasePath, "/cny/capabilities", true, "模拟付款开关状态", capabilities),
	}
}

func hgMallRoutes(h *product.HGHandler) []RouteSpec {
	var list, detail, mine, publish, merchant, apply, pendingMerchants, reviewMerchant, pendingProducts, reviewProduct, adminStatus, image, deleteImage, unusedImages http.HandlerFunc
	var buy, pay, pendingOrders, orders, order, receive, refund, merchantOrders, merchantOrder, ship http.HandlerFunc
	var stageMedia, pendingMedia, reviewMedia, ownPendingMedia, reviewAccess http.HandlerFunc
	var mineDetail, archive, stageEdit, pendingEdits, reviewEdit http.HandlerFunc
	if h != nil {
		list, detail, mine, publish, merchant, apply = h.HGList, h.HGDetail, h.HGMine, h.HGPublish, h.HGMerchant, h.HGApply
		adminStatus = h.HGAdminStatus
		image = h.HGImage
		deleteImage = h.HGDeleteUnusedImage
		unusedImages = h.HGUnusedImages
		pendingMerchants, reviewMerchant, pendingProducts, reviewProduct = h.HGPendingMerchants, h.HGReviewMerchant, h.HGPendingProducts, h.HGReviewProduct
		buy, orders, order, receive, refund = h.HGBuy, h.HGOrders, h.HGOrder, h.HGReceive, h.HGRefund
		pay, pendingOrders = h.HGPay, h.HGPendingOrders
		merchantOrders, merchantOrder, ship = h.HGMerchantOrders, h.HGMerchantOrder, h.HGShip
		stageMedia, pendingMedia, reviewMedia = h.HGStageMedia, h.HGPendingMedia, h.HGReviewMedia
		ownPendingMedia, reviewAccess = h.HGOwnPendingMedia, h.HGReviewAccess
		mineDetail, archive, stageEdit, pendingEdits, reviewEdit = h.HGMineDetail, h.HGArchive, h.HGStageEdit, h.HGPendingEdits, h.HGReviewEdit
	}
	return []RouteSpec{
		NewRouteSpec("mall", http.MethodGet, HGMallModuleBasePath, "/admin-status", true, "当前管理员身份", adminStatus),
		NewRouteSpec("mall", http.MethodPost, HGMallModuleBasePath, "/products/image", true, "上传商品展示图片", image),
		NewRouteSpec("mall", http.MethodPost, HGMallModuleBasePath, "/products/image/delete", true, "删除未使用的商品图片", deleteImage),
		NewRouteSpec("mall", http.MethodGet, HGMallModuleBasePath, "/products/images/unused", true, "本人未发布图片", unusedImages),
		NewRouteSpec("mall", http.MethodGet, HGMallModuleBasePath, "/products", true, "商品目录", list),
		NewRouteSpec("mall", http.MethodGet, HGMallModuleBasePath, "/products/detail", true, "商品详情", detail),
		NewRouteSpec("mall", http.MethodGet, HGMallModuleBasePath, "/products/mine", true, "商家本人商品", mine),
		NewRouteSpec("mall", http.MethodGet, HGMallModuleBasePath, "/products/mine/detail", true, "本人单件商品", mineDetail),
		NewRouteSpec("mall", http.MethodPost, HGMallModuleBasePath, "/products/archive", true, "本人商品归档", archive),
		NewRouteSpec("mall", http.MethodPost, HGMallModuleBasePath, "/products/edit", true, "本人商品文字编辑待审", stageEdit),
		NewRouteSpec("mall", http.MethodPost, HGMallModuleBasePath, "/products/publish", true, "发布单 SKU 商品草稿", publish),
		NewRouteSpec("mall", http.MethodPost, HGMallModuleBasePath, "/products/images/stage", true, "本人已上架商品补图待审", stageMedia),
		NewRouteSpec("mall", http.MethodGet, HGMallModuleBasePath, "/products/images/pending", true, "本人商品待审补图", ownPendingMedia),
		NewRouteSpec("mall", http.MethodGet, HGMallModuleBasePath, "/merchant", true, "本人商家入驻状态", merchant),
		NewRouteSpec("mall", http.MethodPost, HGMallModuleBasePath, "/merchant/apply", true, "申请商家入驻", apply),
		NewRouteSpec("mall", http.MethodGet, HGMallModuleBasePath, "/review/merchants", true, "待审核商家", pendingMerchants, HGPermissionPackage.HGMallMerchantReview),
		NewRouteSpec("mall", http.MethodGet, HGMallModuleBasePath, "/review/access", true, "当前商城审核权限", reviewAccess),
		NewRouteSpec("mall", http.MethodPost, HGMallModuleBasePath, "/review/merchants/decision", true, "审核商家", reviewMerchant, HGPermissionPackage.HGMallMerchantReview),
		NewRouteSpec("mall", http.MethodGet, HGMallModuleBasePath, "/review/products", true, "待审核商品", pendingProducts, HGPermissionPackage.HGMallMerchantReview),
		NewRouteSpec("mall", http.MethodPost, HGMallModuleBasePath, "/review/products/decision", true, "审核商品", reviewProduct, HGPermissionPackage.HGMallMerchantReview),
		NewRouteSpec("mall", http.MethodGet, HGMallModuleBasePath, "/review/images", true, "待审商品补图", pendingMedia, HGPermissionPackage.HGMallMerchantReview),
		NewRouteSpec("mall", http.MethodPost, HGMallModuleBasePath, "/review/images/decision", true, "审核商品补图", reviewMedia, HGPermissionPackage.HGMallMerchantReview),
		NewRouteSpec("mall", http.MethodGet, HGMallModuleBasePath, "/review/edits", true, "待审商品编辑", pendingEdits, HGPermissionPackage.HGMallMerchantReview),
		NewRouteSpec("mall", http.MethodPost, HGMallModuleBasePath, "/review/edits/decision", true, "审核商品编辑", reviewEdit, HGPermissionPackage.HGMallMerchantReview),
		NewRouteSpec("mall", http.MethodPost, HGMallModuleBasePath, "/orders/buy", true, "实物商品预留库存并创建待支付订单", buy),
		NewRouteSpec("mall", http.MethodPost, HGMallModuleBasePath, "/orders/pay", true, "本人待支付订单M币原子支付", pay),
		NewRouteSpec("mall", http.MethodGet, HGMallModuleBasePath, "/orders/pending", true, "本人待支付与超时取消订单", pendingOrders),
		NewRouteSpec("mall", http.MethodGet, HGMallModuleBasePath, "/orders", true, "本人实物订单", orders),
		NewRouteSpec("mall", http.MethodGet, HGMallModuleBasePath, "/orders/detail", true, "本人订单物流", order),
		NewRouteSpec("mall", http.MethodPost, HGMallModuleBasePath, "/orders/receive", true, "确认收货并结算商家", receive),
		NewRouteSpec("mall", http.MethodPost, HGMallModuleBasePath, "/orders/refund", true, "未发货原扣款全额退款", refund),
		NewRouteSpec("mall", http.MethodGet, HGMallModuleBasePath, "/merchant/orders", true, "本店订单", merchantOrders),
		NewRouteSpec("mall", http.MethodGet, HGMallModuleBasePath, "/merchant/orders/detail", true, "本店订单物流", merchantOrder),
		NewRouteSpec("mall", http.MethodPost, HGMallModuleBasePath, "/merchant/orders/ship", true, "本店手动填写运单", ship),
	}
}
