//go:build production

package main

import "testing"

func TestHGRechargeGlobalCatalog(t *testing.T) {
	hgCount := 0
	for _, hgRoute := range collectRouteCatalogs() {
		if hgRoute.Path == "/api/v1/recharge/records" {
			hgCount++
			if hgRoute.Group != "recharge" || hgRoute.Method != "GET" || !hgRoute.NeedAuth {
				t.Fatalf("目录错误: %+v", hgRoute)
			}
		}
	}
	if hgCount != 1 {
		t.Fatalf("全局目录应恰好包含一个充值记录接口: %d", hgCount)
	}
}
