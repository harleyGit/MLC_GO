package HGRouterPackage

import (
	OpsHandlerPackage "MLC_GO/internal/modules/ops/handler"
	"testing"
)

func TestHGRechargeSKURoutes(t *testing.T) {
	for _, handler := range []*OpsHandlerPackage.Handler{nil, OpsHandlerPackage.NewHandler(nil)} {
		want := map[string]bool{"POST /api/v1/ops/payment/recharge/skus": false, "GET /api/v1/ops/payment/recharge/skus/list": false, "POST /api/v1/ops/payment/recharge/skus/update": false, "POST /api/v1/ops/payment/recharge/skus/delete": false}
		for _, route := range opsRoutes(handler) {
			key := route.Method + " " + route.FullPath
			if _, ok := want[key]; ok {
				if !route.NeedAuth || handler != nil && route.Handler == nil {
					t.Fatalf("invalid route %+v", route)
				}
				want[key] = true
			}
		}
		for route, found := range want {
			if !found {
				t.Fatalf("missing %s", route)
			}
		}
	}
}
