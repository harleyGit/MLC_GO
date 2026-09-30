package HGServerPackage

import (
	hgmiddleware "MLC_GO/internal/pkg/middleware"
	"net/http"
)

// HGWalletMethodRules 仅允许明确列出的钱包方法，全部要求JWT。
func HGWalletMethodRules() []hgmiddleware.APIRule {
	return []hgmiddleware.APIRule{
		{Path: "/balance", Version: "v1", Methods: map[string]bool{http.MethodGet: true}, NeedAuth: true},
		{Path: "/recharge/skus", Version: "v1", Methods: map[string]bool{http.MethodGet: true}, NeedAuth: true},
		{Path: "/recharge/orders", Version: "v1", Methods: map[string]bool{http.MethodPost: true}, NeedAuth: true},
		{Path: "/recharge/orders/detail", Version: "v1", Methods: map[string]bool{http.MethodGet: true}, NeedAuth: true},
		{Path: "/recharge/orders/pay", Version: "v1", Methods: map[string]bool{http.MethodPost: true}, NeedAuth: true},
	}
}
