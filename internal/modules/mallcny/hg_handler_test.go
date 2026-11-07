package mallcny

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHGCNYInputAndSandboxBoundary 核对接口在调用数据库前拦截非法 ID 及关闭沙箱时的交易写入。
func TestHGCNYInputAndSandboxBoundary(t *testing.T) {
	t.Setenv("MLC_MALL_CNY_SANDBOX", "false")
	h := HGNewHandler(nil)
	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   string
		fn     http.HandlerFunc
		want   int
	}{
		{"非法商品ID", http.MethodGet, "/cny/products/detail?productId=../../secret", "", h.HGDetail, http.StatusBadRequest},
		{"关闭沙箱不可读取未迁移的人民币目录", http.MethodGet, "/cny/products", "", h.HGList, http.StatusForbidden},
		{"关闭沙箱禁止下单", http.MethodPost, "/cny/orders", "{}", h.HGBuy, http.StatusForbidden},
		{"未登录禁止模拟付款", http.MethodPost, "/cny/orders/pay", "{}", h.HGDebugPay, http.StatusUnauthorized},
		{"未登录禁止模拟退款", http.MethodPost, "/cny/orders/refund", "{}", h.HGDebugRefund, http.StatusUnauthorized},
		{"关闭沙箱禁止发布", http.MethodPost, "/cny/products/publish", "{}", h.HGPublish, http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			tc.fn(w, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
			if w.Code != tc.want {
				t.Fatalf("状态码=%d，预期=%d", w.Code, tc.want)
			}
		})
	}
}

func TestHGCNYQuoteBounds(t *testing.T) {
	r := HGNewRepository(nil)
	price := int64(800)
	product := HGProduct{ProductID: "P", SKUID: "S", PriceFen: 1000, GroupPriceFen: &price, GroupMembers: 2, Available: 4}
	request := HGOrderRequest{ProductID: "P", SKUID: "S", Quantity: 2, PurchaseType: "single"}
	quote, err := r.hgQuote(context.Background(), nil, "buyer", request, product)
	if err != nil || quote.PayableFen != 2000 || !quote.Available {
		t.Fatalf("单买服务端金额错误: %+v %v", quote, err)
	}
	request.PurchaseType = "group_create"
	quote, err = r.hgQuote(context.Background(), nil, "buyer", request, product)
	if err != nil || quote.PayableFen != 1600 {
		t.Fatalf("拼单服务端金额错误: %+v %v", quote, err)
	}
	request.CouponID = "COUPON"
	if _, err = r.hgQuote(context.Background(), nil, "buyer", request, product); !errors.Is(err, ErrHGInvalid) {
		t.Fatalf("拼单叠券未拒绝: %v", err)
	}
	request.CouponID = ""
	request.Quantity = 0
	if _, err = r.hgQuote(context.Background(), nil, "buyer", request, product); !errors.Is(err, ErrHGInvalid) {
		t.Fatalf("无效数量未拒绝: %v", err)
	}
}
