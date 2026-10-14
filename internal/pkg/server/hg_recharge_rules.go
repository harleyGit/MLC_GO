package HGServerPackage

import (
	hgmiddleware "MLC_GO/internal/pkg/middleware"
	"net/http"
)

// HGRechargeMethodRules 仅开放 JWT 认证的充值记录 GET 接口。
func HGRechargeMethodRules() []hgmiddleware.APIRule {
	return []hgmiddleware.APIRule{{Path: "/records", Version: "v1", Methods: map[string]bool{http.MethodGet: true}, NeedAuth: true}}
}
