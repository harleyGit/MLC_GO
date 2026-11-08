package commerce

import (
	q "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func hgTestReserve() HGReserveRequest {
	return HGReserveRequest{ProductID: "CPRD_1", SKUID: "CSKU_1", Quantity: 2, AddressID: "ADDR_1", ExpectedCent: 200, RequestID: "req-1"}
}

func TestHGReserveRefusesChangedPriceBeforeStock(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	r := hgTestReserve()
	mock.ExpectQuery(regexp.QuoteMeta(q.HGCommerceOrderReplaySQL)).WithArgs("buyer", r.RequestID).WillReturnError(sql.ErrNoRows)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(q.HGCommerceOrderProductLockSQL)).WithArgs(r.ProductID).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(q.HGCommerceOrderReplaySQL)).WithArgs("buyer", r.RequestID).WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(regexp.QuoteMeta(q.HGCommerceOrderQuoteSQL)).WithArgs(r.SKUID, r.ProductID).WillReturnRows(sqlmock.NewRows([]string{"merchant_id", "title", "sku_code", "specs_json", "price_cent", "available"}).AddRow("SHOP_1", "商品", "BLACK", `{}`, 101, 10))
	mock.ExpectRollback()
	_, err = HGNewOrderStore(db).HGReserve(context.Background(), "buyer", r)
	if !errors.Is(err, ErrHGConflict) {
		t.Fatalf("报价变化必须拒绝预占：%v", err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGCancelReleasesExactlyOnce(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := HGNewOrderStore(db)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(q.HGCommerceOrderCloseLockSQL)).WithArgs("COMORD_1").WillReturnRows(sqlmock.NewRows([]string{"user_id", "product_id", "sku_code", "quantity", "status", "expire_at"}).AddRow("buyer", "CPRD_1", "BLACK", 2, 1, time.Now().UTC().Add(time.Minute)))
	mock.ExpectExec(regexp.QuoteMeta(q.HGCommerceReservationReleaseSQL)).WithArgs("COMORD_1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(q.HGCommerceStockReleaseSQL)).WithArgs(uint32(2), uint32(2), "CPRD_1", "BLACK", uint32(2)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(q.HGCommerceOrderCloseSQL)).WithArgs("COMORD_1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(q.HGCommerceTradeCloseSQL)).WithArgs("COMORD_1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(q.HGCommerceOrderCloseLockSQL)).WithArgs("COMORD_1").WillReturnRows(sqlmock.NewRows([]string{"user_id", "product_id", "sku_code", "quantity", "status", "expire_at"}).AddRow("buyer", "CPRD_1", "BLACK", 2, 5, time.Now().UTC().Add(time.Minute)))
	mock.ExpectRollback()
	if err = store.HGCancel(context.Background(), "buyer", "COMORD_1"); err != nil {
		t.Fatal(err)
	}
	if err = store.HGCancel(context.Background(), "buyer", "COMORD_1"); err != nil {
		t.Fatal(err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGShipCannotBypassVerifiedPaymentAndShop(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(q.HGCommerceShipLockSQL)).WithArgs("other-shop", "COMORD_1").WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()
	err = HGNewOrderStore(db).HGShip(context.Background(), "other-shop", HGShipRequest{OrderID: "COMORD_1", Carrier: "快递", TrackingNumber: "TRACK-123"})
	if !errors.Is(err, ErrHGForbidden) {
		t.Fatalf("未授权或未验签已付订单不能发货：%v", err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
