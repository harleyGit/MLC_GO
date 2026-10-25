package HGServerPackage

import (
	hgmiddleware "MLC_GO/internal/pkg/middleware"
	"net/http"
)

// HGMallMethodRules 同步商城模块路由目录；发布和审核需要 JWT，审核还需服务端 Ops RBAC。
func HGMallMethodRules() []hgmiddleware.APIRule {
	return []hgmiddleware.APIRule{
		{Path: "/admin-status", Version: "v1", Methods: map[string]bool{http.MethodGet: true}, NeedAuth: true},
		{Path: "/products/image", Version: "v1", Methods: map[string]bool{http.MethodPost: true}, NeedAuth: true},
		{Path: "/products/image/delete", Version: "v1", Methods: map[string]bool{http.MethodPost: true}, NeedAuth: true},
		{Path: "/products/images/unused", Version: "v1", Methods: map[string]bool{http.MethodGet: true}, NeedAuth: true},
		{Path: "/products", Version: "v1", Methods: map[string]bool{http.MethodGet: true}, NeedAuth: true},
		{Path: "/products/detail", Version: "v1", Methods: map[string]bool{http.MethodGet: true}, NeedAuth: true},
		{Path: "/products/mine", Version: "v1", Methods: map[string]bool{http.MethodGet: true}, NeedAuth: true},
		{Path: "/products/mine/detail", Version: "v1", Methods: map[string]bool{http.MethodGet: true}, NeedAuth: true},
		{Path: "/products/archive", Version: "v1", Methods: map[string]bool{http.MethodPost: true}, NeedAuth: true},
		{Path: "/products/edit", Version: "v1", Methods: map[string]bool{http.MethodPost: true}, NeedAuth: true},
		{Path: "/products/publish", Version: "v1", Methods: map[string]bool{http.MethodPost: true}, NeedAuth: true},
		{Path: "/products/images/stage", Version: "v1", Methods: map[string]bool{http.MethodPost: true}, NeedAuth: true},
		{Path: "/products/images/pending", Version: "v1", Methods: map[string]bool{http.MethodGet: true}, NeedAuth: true},
		{Path: "/merchant", Version: "v1", Methods: map[string]bool{http.MethodGet: true}, NeedAuth: true},
		{Path: "/merchant/apply", Version: "v1", Methods: map[string]bool{http.MethodPost: true}, NeedAuth: true},
		{Path: "/review/merchants", Version: "v1", Methods: map[string]bool{http.MethodGet: true}, NeedAuth: true},
		{Path: "/review/access", Version: "v1", Methods: map[string]bool{http.MethodGet: true}, NeedAuth: true},
		{Path: "/review/merchants/decision", Version: "v1", Methods: map[string]bool{http.MethodPost: true}, NeedAuth: true},
		{Path: "/review/products", Version: "v1", Methods: map[string]bool{http.MethodGet: true}, NeedAuth: true},
		{Path: "/review/products/decision", Version: "v1", Methods: map[string]bool{http.MethodPost: true}, NeedAuth: true},
		{Path: "/review/images", Version: "v1", Methods: map[string]bool{http.MethodGet: true}, NeedAuth: true},
		{Path: "/review/images/decision", Version: "v1", Methods: map[string]bool{http.MethodPost: true}, NeedAuth: true},
		{Path: "/review/edits", Version: "v1", Methods: map[string]bool{http.MethodGet: true}, NeedAuth: true},
		{Path: "/review/edits/decision", Version: "v1", Methods: map[string]bool{http.MethodPost: true}, NeedAuth: true},
		{Path: "/orders/buy", Version: "v1", Methods: map[string]bool{http.MethodPost: true}, NeedAuth: true},
		{Path: "/orders/pay", Version: "v1", Methods: map[string]bool{http.MethodPost: true}, NeedAuth: true},
		{Path: "/orders/pending", Version: "v1", Methods: map[string]bool{http.MethodGet: true}, NeedAuth: true},
		{Path: "/orders", Version: "v1", Methods: map[string]bool{http.MethodGet: true}, NeedAuth: true},
		{Path: "/orders/detail", Version: "v1", Methods: map[string]bool{http.MethodGet: true}, NeedAuth: true},
		{Path: "/orders/receive", Version: "v1", Methods: map[string]bool{http.MethodPost: true}, NeedAuth: true},
		{Path: "/orders/refund", Version: "v1", Methods: map[string]bool{http.MethodPost: true}, NeedAuth: true},
		{Path: "/merchant/orders", Version: "v1", Methods: map[string]bool{http.MethodGet: true}, NeedAuth: true},
		{Path: "/merchant/orders/detail", Version: "v1", Methods: map[string]bool{http.MethodGet: true}, NeedAuth: true},
		{Path: "/merchant/orders/ship", Version: "v1", Methods: map[string]bool{http.MethodPost: true}, NeedAuth: true},
	}
}
