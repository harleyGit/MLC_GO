package HGServerPackage

import (
	"MLC_GO/internal/pkg/middleware"
	"net/http"
)

// HGMallMethodRules 同步商城模块路由目录；发布和审核需要 JWT，审核还需服务端 Ops RBAC。
func HGMallMethodRules() []middleware.APIRule {
	return []middleware.APIRule{
		{Path: "/products", Version: "v1", Methods: map[string]bool{http.MethodGet: true}, NeedAuth: true},
		{Path: "/products/detail", Version: "v1", Methods: map[string]bool{http.MethodGet: true}, NeedAuth: true},
		{Path: "/products/mine", Version: "v1", Methods: map[string]bool{http.MethodGet: true}, NeedAuth: true},
		{Path: "/products/publish", Version: "v1", Methods: map[string]bool{http.MethodPost: true}, NeedAuth: true},
		{Path: "/merchant", Version: "v1", Methods: map[string]bool{http.MethodGet: true}, NeedAuth: true},
		{Path: "/merchant/apply", Version: "v1", Methods: map[string]bool{http.MethodPost: true}, NeedAuth: true},
		{Path: "/review/merchants", Version: "v1", Methods: map[string]bool{http.MethodGet: true}, NeedAuth: true},
		{Path: "/review/merchants/decision", Version: "v1", Methods: map[string]bool{http.MethodPost: true}, NeedAuth: true},
		{Path: "/review/products", Version: "v1", Methods: map[string]bool{http.MethodGet: true}, NeedAuth: true},
		{Path: "/review/products/decision", Version: "v1", Methods: map[string]bool{http.MethodPost: true}, NeedAuth: true},
	}
}
