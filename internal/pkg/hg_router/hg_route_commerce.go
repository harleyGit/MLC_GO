package HGRouterPackage

import (
	"MLC_GO/internal/modules/commerce"
	user "MLC_GO/internal/modules/user/middleware"
	permission "MLC_GO/internal/pkg/permission"
	"net/http"
)

// HGCommerceBasePath 与旧 M币商城及 debug CNY 沙箱隔离；生产商品暂不提供购买接口。
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
	var categories, createCategory, create, edit, submit, review, publish, unpublish, archive, list, mine, pending, detail, mineDetail, pendingDetail, image, unused, remove http.HandlerFunc
	if h != nil {
		categories, createCategory = h.HGCategories, h.HGCreateCategory
		create, edit, submit, review, publish, unpublish = h.HGCreateDraft, h.HGEditDraft, h.HGSubmit, h.HGReview, h.HGPublish, h.HGUnpublish
		archive = h.HGArchive
		list, mine, pending, detail, mineDetail, pendingDetail = h.HGList, h.HGMine, h.HGPending, h.HGDetail, h.HGMineDetail, h.HGPendingDetail
		image, unused, remove = h.HGImage, h.HGUnusedImages, h.HGDeleteUnusedImage
	}
	return []RouteSpec{
		NewRouteSpec("commerce", http.MethodGet, HGCommerceBasePath, "/merchant/categories", true, "本人可经营普通类目", categories),
		NewRouteSpec("commerce", http.MethodPost, HGCommerceBasePath, "/admin/categories", true, "平台维护类目", createCategory, permission.HGMallMerchantReview),
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
	}
}
