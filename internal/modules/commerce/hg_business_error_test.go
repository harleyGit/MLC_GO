package commerce

import (
	q "MLC_GO/internal/pkg/mysql/queries"
	resp "MLC_GO/internal/response"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/spf13/viper"
)

func TestHGStockShortageHTTP(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	order := hgTestReserve()
	mock.ExpectQuery(q.HGCommerceOrderQuoteSQL).WithArgs(order.SKUID, order.ProductID).WillReturnRows(sqlmock.NewRows([]string{"merchant", "title", "sku", "specs", "price", "available"}).AddRow("shop", "商品", "sku", "{}", 100, 1))
	_, err = HGNewOrderStore(db).HGPreview(context.Background(), "buyer", order)
	if !errors.Is(err, ErrHGStock) {
		t.Fatalf("旧哨兵丢失: %v", err)
	}
	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("Accept", resp.HGBusinessMediaType)
	w := httptest.NewRecorder()
	hgError(w, req, err)
	var b resp.HGBusinessEnvelope
	if json.Unmarshal(w.Body.Bytes(), &b) != nil || w.Code != 409 || b.Code != 1402004 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGSimulatedPaymentCallbackReplay(t *testing.T) {
	t.Setenv("SERVER_ENV", "debug")
	t.Setenv("MLC_COMMERCE_DEBUG_TRADING", "true")
	old := viper.Get("runtime.loaded_env")
	viper.Set("runtime.loaded_env", "debug")
	t.Cleanup(func() { viper.Set("runtime.loaded_env", old) })
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	event := HGSimulatedEvent{EventID: "evt", PaymentID: "payment", OrderID: "order", ExternalPaymentID: "external", Currency: "CNY", AmountCent: 100, Status: "SUCCESS"}
	raw, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	key := make([]byte, 32) // 隔离验签夹具，不连接任何支付渠道。
	hash := sha256.Sum256(raw)
	mock.ExpectBegin()
	mock.ExpectQuery(q.HGCommerceSimOrderLockSQL).WithArgs("order").WillReturnRows(sqlmock.NewRows([]string{"owner", "product", "sku", "qty", "status", "expiry", "amount", "merchant"}).AddRow("u", "p", "s", 1, 2, time.Now(), 100, "m"))
	mock.ExpectQuery(q.HGCommerceSimEventReadSQL).WithArgs("evt").WillReturnRows(sqlmock.NewRows([]string{"hash", "result"}).AddRow(hash[:], "PAID"))
	mock.ExpectRollback() // 已入账的回调只读回执，无第二次写入。
	got, err := HGNewOrderStore(db).HGApplySimulatedEvent(context.Background(), key, raw, HGSimulatedSignature(key, raw))
	if err != nil || got.Result != "PAID" {
		t.Fatalf("重放结果 %+v: %v", got, err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
