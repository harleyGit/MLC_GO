//go:build production

package main

import "testing"

func TestHGMallRoutesIncludedInRootCatalog(t *testing.T) {
	needed := map[string]bool{
		"/api/v1/mall/products":             false,
		"/api/v1/mall/orders/buy":           false,
		"/api/v1/mall/orders/refund":        false,
		"/api/v1/mall/merchant/orders/ship": false,
	}
	for _, route := range collectRouteCatalogs() {
		if _, ok := needed[route.Path]; ok {
			needed[route.Path] = true
		}
	}
	for path, found := range needed {
		if !found {
			t.Errorf("根路由清单缺少商城接口 %s", path)
		}
	}
}
