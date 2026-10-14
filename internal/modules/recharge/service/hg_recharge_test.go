package service

import (
	"MLC_GO/internal/modules/recharge/model"
	"context"
	"strings"
	"testing"
	"time"
)

type hgRechargeRepo struct {
	hgItems  []model.HGRecord
	hgErr    error
	hgUser   string
	hgCursor *model.HGCursor
	hgLimit  int
}

func (hgRepo *hgRechargeRepo) HGList(_ context.Context, hgUser string, hgCursor *model.HGCursor, hgLimit int) ([]model.HGRecord, error) {
	hgRepo.hgUser, hgRepo.hgCursor, hgRepo.hgLimit = hgUser, hgCursor, hgLimit
	return hgRepo.hgItems, hgRepo.hgErr
}

func TestHGParsePageStrictBounds(t *testing.T) {
	for _, hgText := range []string{"0", "-1", "51", "01", "abc", "+1", "1.0", " 1", strings.Repeat("9", 1000)} {
		if _, _, hgErr := hgParsePage("", hgText); hgErr == nil {
			t.Fatalf("pageSize %q should fail", hgText)
		}
	}
	if hgCursor, hgLimit, hgErr := hgParsePage("0", ""); hgErr != nil || hgCursor != nil || hgLimit != 50 {
		t.Fatalf("first page parse: %#v %d %v", hgCursor, hgLimit, hgErr)
	}
	if _, _, hgErr := hgParsePage(strings.Repeat("A", 22), ""); hgErr == nil {
		t.Fatal("invalid cursor should fail")
	}
	for _, hgText := range []string{"bad", strings.Repeat("A", 10000), strings.Repeat("/", 22), strings.Repeat("A", 20) + "==", strings.Repeat("A", 21) + "B"} {
		if _, _, hgErr := hgParsePage(hgText, ""); hgErr == nil {
			t.Fatal("非法游标未拒绝")
		}
	}
}

func TestHGRecordsMapsStatusesAndPreservesAmounts(t *testing.T) {
	hgNow := time.Date(2026, 10, 10, 1, 0, 0, 0, time.UTC)
	hgCreated := hgNow.Add(-time.Second)
	hgRepo := &hgRechargeRepo{hgItems: []model.HGRecord{
		{HGID: 1, HGOrderID: "pending", HGTitle: "标题", HGCurrency: "CNY", HGPayAmount: 9007199254740993, HGTotalCoin: 1844674407370955161, HGStatus: 1, HGPaymentMode: "unavailable", HGCreatedAt: hgCreated, HGExpiresAt: hgNow.Add(time.Minute)},
		{HGID: 2, HGOrderID: "same-ms", HGCurrency: "CNY", HGPayAmount: 2, HGTotalCoin: 3, HGStatus: 2, HGCreatedAt: hgCreated, HGExpiresAt: hgNow},
		{HGID: 3, HGOrderID: "paid", HGCurrency: "CNY", HGPayAmount: 4, HGTotalCoin: 5, HGStatus: 3, HGCreatedAt: hgCreated.Add(-time.Millisecond), HGExpiresAt: hgNow, HGPaidAt: &hgNow},
		{HGID: 4, HGOrderID: "cancelled", HGCurrency: "CNY", HGPayAmount: 6, HGTotalCoin: 7, HGStatus: 4, HGCreatedAt: hgCreated.Add(-2 * time.Millisecond), HGExpiresAt: hgNow},
		{HGID: 5, HGOrderID: "refunding", HGCurrency: "CNY", HGPayAmount: 8, HGTotalCoin: 9, HGStatus: 5, HGCreatedAt: hgCreated.Add(-3 * time.Millisecond), HGExpiresAt: hgNow},
		{HGID: 6, HGOrderID: "refunded", HGCurrency: "CNY", HGPayAmount: 10, HGTotalCoin: 11, HGStatus: 6, HGCreatedAt: hgCreated.Add(-4 * time.Millisecond), HGExpiresAt: hgNow},
	}, hgLimit: 50}
	hgService := HGNewService(hgRepo)
	hgService.hgNow = func() time.Time { return hgNow }
	hgPage, hgErr := hgService.HGRecords(context.Background(), "user-A", "", "")
	if hgErr != nil || hgRepo.hgUser != "user-A" || len(hgPage.HGList) != 6 {
		t.Fatalf("page=%+v err=%v", hgPage, hgErr)
	}
	if hgPage.HGList[0].HGPayAmount != "9007199254740993" || hgPage.HGList[0].HGTotalCoin != "1844674407370955161" || hgPage.HGList[0].HGStatus != "pending" {
		t.Fatalf("precision/status: %+v", hgPage.HGList[0])
	}
	for hgIndex, hgWant := range []string{"pending", "processing", "paid", "cancelled", "refunding", "refunded"} {
		if hgPage.HGList[hgIndex].HGStatus != hgWant {
			t.Fatalf("status[%d]=%s", hgIndex, hgPage.HGList[hgIndex].HGStatus)
		}
	}
}

func TestHGRecordsUsesLimitPlusOneAndSameMillisecondCursor(t *testing.T) {
	hgCreated := time.Date(2026, 10, 10, 1, 0, 0, 123000000, time.UTC)
	hgRepo := &hgRechargeRepo{hgItems: make([]model.HGRecord, 51)}
	for hgIndex := range hgRepo.hgItems {
		hgRepo.hgItems[hgIndex] = model.HGRecord{HGID: uint64(51 - hgIndex), HGCurrency: "CNY", HGCreatedAt: hgCreated, HGExpiresAt: hgCreated.Add(time.Minute)}
	}
	hgPage, hgErr := HGNewService(hgRepo).HGRecords(context.Background(), "owner", "", "50")
	if hgErr != nil || len(hgPage.HGList) != 50 || !hgPage.HGHasMore || hgPage.HGNextCursor == "" || hgRepo.hgLimit != 50 {
		t.Fatalf("page=%+v limit=%d err=%v", hgPage, hgRepo.hgLimit, hgErr)
	}
	hgCursor, hgLimit, hgErr := hgParsePage(hgPage.HGNextCursor, "50")
	if hgErr != nil || hgLimit != 50 || hgCursor.HGCreatedAt.UnixMilli() != hgCreated.UnixMilli() || hgCursor.HGID != 2 {
		t.Fatalf("cursor=%+v limit=%d err=%v", hgCursor, hgLimit, hgErr)
	}
	hgRepo.hgItems = hgRepo.hgItems[50:]
	hgNext, hgErr := HGNewService(hgRepo).HGRecords(context.Background(), "owner", hgPage.HGNextCursor, "50")
	if hgErr != nil || hgRepo.hgCursor.HGID != 2 || len(hgNext.HGList) != 1 || hgNext.HGHasMore || hgNext.HGNextCursor != "" {
		t.Fatalf("续页异常: %+v %v", hgNext, hgErr)
	}
	hgRepo.hgItems = make([]model.HGRecord, 50)
	hgPage, hgErr = HGNewService(hgRepo).HGRecords(context.Background(), "owner", "0", "50")
	if hgErr != nil || len(hgPage.HGList) != 50 || hgPage.HGHasMore || hgPage.HGNextCursor != "" {
		t.Fatalf("50条终页异常: %+v %v", hgPage, hgErr)
	}
}

func TestHGRecordsExpirationBoundary(t *testing.T) {
	hgNow := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	for _, hgStatus := range []int{0, 1} {
		for _, hgDelta := range []time.Duration{-time.Millisecond, 0, time.Millisecond} {
			hgRepo := &hgRechargeRepo{hgItems: []model.HGRecord{{HGStatus: hgStatus, HGExpiresAt: hgNow.Add(hgDelta)}}}
			hgService := HGNewService(hgRepo)
			hgService.hgNow = func() time.Time { return hgNow }
			hgPage, hgErr := hgService.HGRecords(context.Background(), "owner", "", "")
			hgWant := "expired"
			if hgDelta > 0 {
				hgWant = "pending"
			}
			if hgErr != nil || hgPage.HGList[0].HGStatus != hgWant {
				t.Fatalf("截止时间映射异常: %+v %v", hgPage, hgErr)
			}
		}
	}
}

func TestHGRecordsFailsClosedUnknownStatus(t *testing.T) {
	hgRepo := &hgRechargeRepo{hgItems: []model.HGRecord{{HGStatus: 7, HGCreatedAt: time.Now(), HGExpiresAt: time.Now()}}}
	if _, hgErr := HGNewService(hgRepo).HGRecords(context.Background(), "owner", "", ""); hgErr != model.HGErrInvalidRecord {
		t.Fatalf("err=%v", hgErr)
	}
}
