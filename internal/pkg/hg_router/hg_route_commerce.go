package HGRouterPackage

import (
	"MLC_GO/internal/modules/commerce"
	user "MLC_GO/internal/modules/user/middleware"
	permission "MLC_GO/internal/pkg/permission"
	"net/http"
)

// HGCommerceBasePath 与旧 M币商城及 debug CNY 沙箱隔离；交易写路由在 pre/prod 均 fail-closed。
const HGCommerceBasePath = "/api/v1/commerce"

// HGNewCommerceRouteGroup 复用 JWT、API Guard、签名及商城 IAM 权限校验。
func HGNewCommerceRouteGroup(h *commerce.HGHandler, authorizer HGRouteAuthorizer) http.Handler {
	group := NewRouteGroup(RouteGroupConfig{BasePath: HGCommerceBasePath, AuthMiddleware: user.AuthMiddleware, Authorizer: authorizer}, hgCommerceRoutes(h))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		group.ServeHTTP(w, r)
	})
}

// HGCommerceRouteCatalog 返回与实际注册一致的生产商品路径及权限声明。
func HGCommerceRouteCatalog() []RouteCatalogItem {
	return BuildRouteCatalogItems(hgCommerceRoutes(nil))
}

func hgCommerceRoutes(h *commerce.HGHandler) []RouteSpec {
	var categories, adminCategories, createCategory, deleteCategory, categoryImage, categoryUnused, categoryRemove, create, edit, submit, review, publish, unpublish, archive, list, mine, pending, detail, mineDetail, pendingDetail, image, unused, remove http.HandlerFunc
	var inventory, adjust, preview, createOrder, orderDetail, orders, cancelOrder, payment, refund, callback, settle, reconcile, merchantOrders, merchantDetail, shipment http.HandlerFunc
	if h != nil {
		categories, createCategory = h.HGCategories, h.HGCreateCategory
		adminCategories = h.HGAdminCategories
		deleteCategory = h.HGDeleteCategory
		categoryImage = h.HGUploadCategoryImage
		categoryUnused, categoryRemove = h.HGUnusedCategoryImages, h.HGDeleteUnusedCategoryImage
		create, edit, submit, review, publish, unpublish = h.HGCreateDraft, h.HGEditDraft, h.HGSubmit, h.HGReview, h.HGPublish, h.HGUnpublish
		archive = h.HGArchive
		list, mine, pending, detail, mineDetail, pendingDetail = h.HGList, h.HGMine, h.HGPending, h.HGDetail, h.HGMineDetail, h.HGPendingDetail
		image, unused, remove = h.HGImage, h.HGUnusedImages, h.HGDeleteUnusedImage
		inventory, adjust, preview, createOrder = h.HGInventory, h.HGInventoryAdjust, h.HGOrderPreview, h.HGOrderCreate
		orderDetail, orders, cancelOrder, payment, shipment = h.HGOrderDetail, h.HGOrderList, h.HGOrderCancel, h.HGOrderPayment, h.HGOrderShipment
		refund, callback, merchantOrders, merchantDetail = h.HGOrderRefund, h.HGSimulatedCallback, h.HGMerchantOrders, h.HGMerchantOrderDetail
		reconcile = h.HGSimulatedReconcile
		settle = h.HGSimulatedSettle
	}
	return []RouteSpec{
		NewRouteSpec("commerce", http.MethodGet, HGCommerceBasePath, "/merchant/categories", true, "本人可经营普通类目", categories),
		NewRouteSpec("commerce", http.MethodGet, HGCommerceBasePath, "/admin/categories", true, "平台普通叶子类目", adminCategories, permission.HGMallMerchantReview),
		NewRouteSpec("commerce", http.MethodPost, HGCommerceBasePath, "/admin/categories", true, "平台维护类目", createCategory, permission.HGMallMerchantReview),
		NewRouteSpec("commerce", http.MethodPost, HGCommerceBasePath, "/admin/categories/delete", true, "平台停用类目", deleteCategory, permission.HGMallMerchantReview),
		NewRouteSpec("commerce", http.MethodPost, HGCommerceBasePath, "/admin/categories/images", true, "管理员上传类目图片", categoryImage, permission.HGMallMerchantReview),
		NewRouteSpec("commerce", http.MethodGet, HGCommerceBasePath, "/admin/categories/images/unused", true, "本人未绑定类目图片", categoryUnused, permission.HGMallMerchantReview),
		NewRouteSpec("commerce", http.MethodPost, HGCommerceBasePath, "/admin/categories/images/delete", true, "清理本人未绑定类目图片", categoryRemove, permission.HGMallMerchantReview),
		NewRouteSpec("commerce", http.MethodPost, HGCommerceBasePath, "/merchant/products", true, "创建 CNY 商品草稿", create, permission.HGMallProductCreate),
		NewRouteSpec("commerce", http.MethodPost, HGCommerceBasePath, "/merchant/products/edit", true, "本人编辑草稿版本", edit, permission.HGMallProductUpdate),
		NewRouteSpec("commerce", http.MethodPost, HGCommerceBasePath, "/merchant/products/submit", true, "商品提交审核", submit, permission.HGMallProductPublish),
		NewRouteSpec("commerce", http.MethodPost, HGCommerceBasePath, "/admin/products/decision", true, "商品审批", review, permission.HGMallMerchantReview),
		NewRouteSpec("commerce", http.MethodPost, HGCommerceBasePath, "/merchant/products/publish", true, "本人商品上架", publish, permission.HGMallProductPublish),
		NewRouteSpec("commerce", http.MethodPost, HGCommerceBasePath, "/merchant/products/unpublish", true, "本人商品下架", unpublish, permission.HGMallProductPublish),
		NewRouteSpec("commerce", http.MethodPost, HGCommerceBasePath, "/merchant/products/archive", true, "本人商品永久归档", archive, permission.HGMallProductUpdate),
		NewRouteSpec("commerce", http.MethodGet, HGCommerceBasePath, "/products", true, "生产 CNY 已上架目录", list),
		NewRouteSpec("commerce", http.MethodGet, HGCommerceBasePath, "/products/detail", true, "生产 CNY 商品公开详情", detail),
		NewRouteSpec("commerce", http.MethodGet, HGCommerceBasePath, "/merchant/products", true, "本人商品游标列表", mine),
		NewRouteSpec("commerce", http.MethodGet, HGCommerceBasePath, "/merchant/products/detail", true, "本人草稿详情", mineDetail),
		NewRouteSpec("commerce", http.MethodGet, HGCommerceBasePath, "/admin/products/pending", true, "待审商品游标列表", pending, permission.HGMallMerchantReview),
		NewRouteSpec("commerce", http.MethodGet, HGCommerceBasePath, "/admin/products/detail", true, "待审商品审核详情", pendingDetail, permission.HGMallMerchantReview),
		NewRouteSpec("commerce", http.MethodPost, HGCommerceBasePath, "/merchant/images", true, "本人上传生产商品图片", image, permission.HGMallProductCreate),
		NewRouteSpec("commerce", http.MethodGet, HGCommerceBasePath, "/merchant/images/unused", true, "本人未绑定图片", unused),
		NewRouteSpec("commerce", http.MethodPost, HGCommerceBasePath, "/merchant/images/delete", true, "清理本人未绑定图片", remove, permission.HGMallProductCreate),
		NewRouteSpec("commerce", http.MethodGet, HGCommerceBasePath, "/merchant/inventory", true, "本店 debug SKU 库存", inventory, permission.HGMallProductUpdate),
		NewRouteSpec("commerce", http.MethodPost, HGCommerceBasePath, "/merchant/inventory/adjust", true, "本店 debug 有审计的库存盘点", adjust, permission.HGMallProductUpdate),
		NewRouteSpec("commerce", http.MethodPost, HGCommerceBasePath, "/orders/preview", true, "debug 服务端报价", preview),
		NewRouteSpec("commerce", http.MethodPost, HGCommerceBasePath, "/orders", true, "debug 待支付订单与库存预占", createOrder),
		NewRouteSpec("commerce", http.MethodGet, HGCommerceBasePath, "/orders/detail", true, "本人生产 CNY 订单详情", orderDetail),
		NewRouteSpec("commerce", http.MethodGet, HGCommerceBasePath, "/orders", true, "本人生产 CNY 订单列表", orders),
		NewRouteSpec("commerce", http.MethodPost, HGCommerceBasePath, "/orders/cancel", true, "本人待支付订单取消", cancelOrder),
		NewRouteSpec("commerce", http.MethodPost, HGCommerceBasePath, "/orders/payment", true, "debug 模拟支付尝试", payment),
		NewRouteSpec("commerce", http.MethodPost, HGCommerceBasePath, "/orders/refund", true, "debug 模拟已付未发货退款", refund),
		NewRouteSpec("commerce", http.MethodPost, HGCommerceBasePath, "/admin/simulated-payment/callback", true, "隔离 debug 已验签模拟渠道回调", callback, permission.HGMallMerchantReview),
		NewRouteSpec("commerce", http.MethodPost, HGCommerceBasePath, "/admin/simulated-payment/settle", true, "隔离 debug 管理员生成一次模拟收款事件", settle, permission.HGMallMerchantReview),
		NewRouteSpec("commerce", http.MethodGet, HGCommerceBasePath, "/admin/simulated-payment/reconcile", true, "隔离 debug 模拟订单对账漂移报告", reconcile, permission.HGMallMerchantReview),
		NewRouteSpec("commerce", http.MethodGet, HGCommerceBasePath, "/merchant/orders", true, "本店已付订单列表", merchantOrders, permission.HGMallOrderRead),
		NewRouteSpec("commerce", http.MethodGet, HGCommerceBasePath, "/merchant/orders/detail", true, "本店已付订单详情", merchantDetail, permission.HGMallOrderRead),
		NewRouteSpec("commerce", http.MethodPost, HGCommerceBasePath, "/merchant/orders/ship", true, "debug 已付本店订单发货", shipment, permission.HGMallOrderShip),
	}
}
