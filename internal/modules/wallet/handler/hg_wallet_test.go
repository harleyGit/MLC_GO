package handler

import (
	hgjwt "MLC_GO/internal/modules/user/middleware"
	hguser "MLC_GO/internal/modules/user/service"
	"MLC_GO/internal/modules/wallet/model"
	"MLC_GO/internal/modules/wallet/service"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type hgStore struct{ hgCalls int }

func (hgStore *hgStore) HGListCandidates(context.Context, uint64) ([]model.HGSKU, error) {
	hgStore.hgCalls++
	return nil, nil
}
func (hgStore *hgStore) HGCreate(context.Context, string, string, string, time.Time) (model.HGOrder, error) {
	hgStore.hgCalls++
	return model.HGOrder{}, model.HGErrUnsupported
}
func (hgStore *hgStore) HGDetail(hgCtx context.Context, hgUser, hgOrder string) (model.HGOrder, error) {
	hgStore.hgCalls++
	if hgUser != "owner" || hgOrder != "order" {
		return model.HGOrder{}, model.HGErrNotFound
	}
	return model.HGOrder{HGOrderID: "order", HGUserID: "owner", HGDisplayName: "昵称", HGCurrency: "CNY", HGPayAmount: 600, HGTotalCoin: 50, HGExpiresAt: time.Now().Add(time.Minute)}, nil
}

func TestHGWalletHTTPBoundary(t *testing.T) {
	hgStore := &hgStore{}
	hgHandler := HGNewHandler(service.HGNewService(hgStore, nil))
	for _, hgCase := range []struct {
		hgName, hgUser, hgBody string
		hgHandler              http.HandlerFunc
		hgStatus               int
		hgWant                 string
	}{
		{"未登录余额", "", "", hgHandler.HGBalance, 401, "300001"},
		{"未登录目录", "", "", hgHandler.HGSKUs, 401, "300001"},
		{"未登录订单", "", "", hgHandler.HGOrderDetail, 401, "300001"},
		{"伪造金额", "owner", `{"skuId":"sku","requestId":"request","payAmount":1}`, hgHandler.HGCreateOrder, 400, "100001"},
		{"多JSON", "owner", `{"skuId":"sku","requestId":"request"}{}`, hgHandler.HGCreateOrder, 400, "100001"},
		{"超大请求体", "owner", `{"skuId":"` + strings.Repeat("a", 17<<10) + `"}`, hgHandler.HGCreateOrder, 400, "100001"},
		{"超过币数上限", "owner", `{"skuId":"sku","requestId":"request"}`, hgHandler.HGCreateOrder, 422, "1000"},
		{"渠道未配置", "owner", `{"orderId":"order"}`, hgHandler.HGPay, 503, "支付渠道未配置，暂不能付款"},
		{"他人订单", "other", `{"orderId":"order"}`, hgHandler.HGPay, 404, "订单不存在或无权访问"},
		{"不存在订单", "owner", `{"orderId":"missing"}`, hgHandler.HGPay, 404, "订单不存在或无权访问"},
	} {
		t.Run(hgCase.hgName, func(t *testing.T) {
			hgRequest := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(hgCase.hgBody))
			if hgCase.hgUser != "" {
				hgRequest = hgRequest.WithContext(context.WithValue(hgRequest.Context(), hgjwt.UserIDKey, &hguser.HGClaims{UserID: hgCase.hgUser}))
			}
			hgRecorder := httptest.NewRecorder()
			hgCase.hgHandler(hgRecorder, hgRequest)
			if hgRecorder.Code != hgCase.hgStatus || !strings.Contains(hgRecorder.Body.String(), hgCase.hgWant) {
				t.Fatalf("status=%d body=%s", hgRecorder.Code, hgRecorder.Body.String())
			}
		})
	}
	if hgStore.hgCalls != 4 {
		t.Fatalf("无效请求不应访问仓储: %d", hgStore.hgCalls)
	}
}

type hgBalanceReader uint64

func (hgValue hgBalanceReader) Balance(context.Context, string) (uint64, error) {
	return uint64(hgValue), nil
}

func TestHGWalletBalanceJSONPrecision(t *testing.T) {
	for _, hgCase := range []struct {
		hgValue uint64
		hgWant  string
	}{
		{0, "0"}, {9007199254740993, "9007199254740993"}, {^uint64(0), "18446744073709551615"},
	} {
		hgHandler := HGNewHandler(service.HGNewService(nil, hgBalanceReader(hgCase.hgValue)))
		hgRequest := httptest.NewRequest(http.MethodGet, "/balance", nil)
		hgRequest = hgRequest.WithContext(context.WithValue(hgRequest.Context(), hgjwt.UserIDKey, &hguser.HGClaims{UserID: "owner"}))
		hgRecorder := httptest.NewRecorder()
		hgHandler.HGBalance(hgRecorder, hgRequest)
		var hgResponse struct {
			Result struct {
				Balance string `json:"balance"`
			} `json:"result"`
		}
		if hgErr := json.Unmarshal(hgRecorder.Body.Bytes(), &hgResponse); hgErr != nil || hgRecorder.Code != 200 || hgResponse.Result.Balance != hgCase.hgWant {
			t.Fatalf("余额必须无损编码为字符串: %s, %v", hgRecorder.Body.String(), hgErr)
		}
	}
}

func TestHGWalletChineseErrors(t *testing.T) {
	for _, hgCase := range []struct {
		hgErr     error
		hgStatus  int
		hgMessage string
	}{
		{model.HGErrInvalid, 400, "钱包参数无效"},
		{model.HGErrConflict, 409, "requestId已绑定其他充值档位"},
		{model.HGErrSKUUnavailable, 404, "充值档位不可用"},
		{model.HGErrUnsupported, 422, "暂不支持单笔超过1000币的充值"},
		{errors.New("内部数据库诊断不可外泄"), 500, "钱包服务暂不可用"},
	} {
		hgRecorder := httptest.NewRecorder()
		hgWalletError(hgRecorder, httptest.NewRequest(http.MethodGet, "/", nil), hgCase.hgErr)
		var hgResponse struct {
			Message string `json:"message"`
		}
		if hgErr := json.Unmarshal(hgRecorder.Body.Bytes(), &hgResponse); hgErr != nil || hgRecorder.Code != hgCase.hgStatus || hgResponse.Message != hgCase.hgMessage {
			t.Fatalf("中文错误不匹配: %s, %v", hgRecorder.Body.String(), hgErr)
		}
	}
}
