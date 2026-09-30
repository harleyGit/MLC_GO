package service

import (
	"MLC_GO/internal/modules/wallet/dto"
	"MLC_GO/internal/modules/wallet/model"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type hgStore struct {
	hgItems                       []model.HGSKU
	hgOrder                       model.HGOrder
	hgCursor                      uint64
	hgCreateCalls, hgBalanceCalls int
	hgErr                         error
}

func (hgStore *hgStore) HGListCandidates(hgCtx context.Context, hgCursor uint64) ([]model.HGSKU, error) {
	hgStore.hgCursor = hgCursor
	return hgStore.hgItems, hgStore.hgErr
}
func (hgStore *hgStore) HGCreate(hgCtx context.Context, hgUser, hgSKU, hgRequest string, hgNow time.Time) (model.HGOrder, error) {
	hgStore.hgCreateCalls++
	hgStore.hgOrder.HGUserID = hgUser
	hgStore.hgOrder.HGSKUID = hgSKU
	hgStore.hgOrder.HGRequestID = hgRequest
	hgStore.hgOrder.HGCreatedAt = hgNow
	return hgStore.hgOrder, hgStore.hgErr
}
func (hgStore *hgStore) HGDetail(hgCtx context.Context, hgUser, hgOrder string) (model.HGOrder, error) {
	if hgUser != hgStore.hgOrder.HGUserID || hgOrder != hgStore.hgOrder.HGOrderID {
		return model.HGOrder{}, model.HGErrNotFound
	}
	return hgStore.hgOrder, hgStore.hgErr
}
func (hgStore *hgStore) Balance(hgCtx context.Context, hgUser string) (uint64, error) {
	hgStore.hgBalanceCalls++
	if _, hgOK := hgCtx.Deadline(); !hgOK {
		return 0, errors.New("缺少超时")
	}
	return 123, hgStore.hgErr
}

func TestHGWalletPage(t *testing.T) {
	hgNow := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
	hgStore := &hgStore{hgItems: []model.HGSKU{
		{HGID: 1, HGSKUID: "expired", HGEndTime: &hgNow},
		{HGID: 2, HGSKUID: "future", HGStartTime: hgNow.Add(time.Second)},
		{HGID: 3, HGSKUID: "valid", HGStartTime: hgNow, HGTotalCoin: 1000},
		{HGID: 4, HGSKUID: "large", HGTotalCoin: 1001},
	}}
	hgService := HGNewService(hgStore, hgStore)
	hgService.hgNow = func() time.Time { return hgNow }
	hgPage, hgErr := hgService.HGListSKUs(context.Background(), "0", "1")
	if hgErr != nil || len(hgPage.HGList) != 1 || hgPage.HGList[0].HGSKUID != "valid" || !hgPage.HGHasMore || hgPage.HGNextCursor != "3" || !hgPage.HGList[0].HGSupported || hgPage.HGList[0].HGPaymentAvailable {
		t.Fatalf("page=%+v err=%v", hgPage, hgErr)
	}
	hgStore.hgItems = hgStore.hgItems[3:]
	hgPage, hgErr = hgService.HGListSKUs(context.Background(), hgPage.HGNextCursor, "1000")
	if hgErr != nil || hgStore.hgCursor != 3 || hgPage.HGHasMore || hgPage.HGNextCursor != "" || hgPage.HGList[0].HGSupported {
		t.Fatalf("page=%+v err=%v", hgPage, hgErr)
	}
	hgStore.hgItems = make([]model.HGSKU, 201)
	for hgIndex := range hgStore.hgItems {
		hgStore.hgItems[hgIndex] = model.HGSKU{HGID: uint64(hgIndex + 1), HGEndTime: &hgNow}
	}
	hgPage, hgErr = hgService.HGListSKUs(context.Background(), "", "")
	if hgErr != nil || len(hgPage.HGList) != 0 || !hgPage.HGHasMore || hgPage.HGNextCursor != "201" {
		t.Fatalf("空窗口必须可续页: %+v %v", hgPage, hgErr)
	}
	for _, hgCase := range [][2]string{{"-1", ""}, {"abc", "20"}, {"18446744073709551616", "1"}, {"", "0"}, {"", "-1"}, {"", "2.5"}} {
		if _, hgErr := hgService.HGListSKUs(context.Background(), hgCase[0], hgCase[1]); !errors.Is(hgErr, model.HGErrInvalid) {
			t.Fatal(hgCase, hgErr)
		}
	}
}

func TestHGWalletOrderAndPayment(t *testing.T) {
	hgNow := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
	hgStore := &hgStore{hgOrder: model.HGOrder{HGOrderID: "order", HGUserID: "owner", HGDisplayName: "快照昵称", HGTitle: "快照标题", HGDescription: "快照说明", HGPayAmount: 600, HGTotalCoin: 50, HGCurrency: "CNY", HGExpiresAt: hgNow.Add(10 * time.Minute)}}
	hgService := HGNewService(hgStore, hgStore)
	hgService.hgNow = func() time.Time { return hgNow }
	hgOrder, hgErr := hgService.HGCreateOrder(context.Background(), "owner", dto.HGCreateRequest{HGSKUID: "sku", HGRequestID: "request"})
	if hgErr != nil || hgOrder.HGStatus != "pending" || hgOrder.HGPayAmount != 600 || hgOrder.HGTotalCoin != 50 || hgOrder.HGDisplayName != "快照昵称" || hgOrder.HGPaymentAvailable {
		t.Fatalf("order=%+v err=%v", hgOrder, hgErr)
	}
	hgNow = hgNow.Add(10 * time.Minute)
	hgOrder, hgErr = hgService.HGOrderDetail(context.Background(), "owner", "order")
	if hgErr != nil || hgOrder.HGStatus != "expired" {
		t.Fatal(hgOrder, hgErr)
	}
	if hgErr := hgService.HGPay(context.Background(), "owner", "order"); !errors.Is(hgErr, model.HGErrPaymentUnavailable) {
		t.Fatal(hgErr)
	}
	for _, hgUser := range []string{"other", ""} {
		if hgErr := hgService.HGPay(context.Background(), hgUser, "order"); !errors.Is(hgErr, model.HGErrNotFound) {
			t.Fatal(hgErr)
		}
	}
	if hgStore.hgBalanceCalls != 0 || hgStore.hgCreateCalls != 1 {
		t.Fatal("付款不应读取余额或新建订单")
	}
	hgBalance, hgErr := hgService.HGBalance(context.Background(), "owner")
	if hgErr != nil || hgBalance.HGBalance != "123" || hgStore.hgBalanceCalls != 1 {
		t.Fatal(hgBalance, hgErr)
	}
	for _, hgRequest := range []string{"", " ", "a b", "中文", strings.Repeat("a", 65)} {
		if _, hgErr := hgService.HGCreateOrder(context.Background(), "owner", dto.HGCreateRequest{HGSKUID: "sku", HGRequestID: hgRequest}); !errors.Is(hgErr, model.HGErrInvalid) {
			t.Fatal(hgRequest, hgErr)
		}
	}
	if hgStore.hgCreateCalls != 1 {
		t.Fatal("无效参数访问仓储")
	}
}
