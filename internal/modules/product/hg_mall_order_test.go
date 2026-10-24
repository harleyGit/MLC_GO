package product

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"errors"
	"math"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestHGBuyChecksStockBeforeWalletMutation(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	req := HGBuyRequest{ProductID: "PRD_1", RequestID: "buy-1", Quantity: 2, ReceiverName: "王明", ReceiverPhone: "13800000000", ReceiverAddress: "上海市收货地址"}
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGMallOrderByRequestSQL)).WithArgs("buyer", req.RequestID).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGMallBuyProductSQL)).WithArgs(req.ProductID).WillReturnRows(sqlmock.NewRows([]string{"product_id", "title", "merchant_id", "user_id", "sku_id", "sku_title", "amount", "available"}).AddRow(req.ProductID, "商品", "MER_1", "seller", "SKU_1", "标准版", 50, 1))
	mock.ExpectRollback()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGMallOrderByRequestSQL)).WithArgs("buyer", req.RequestID).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	repo := HGNewOrderRepository(db)
	_, err = repo.Buy(context.Background(), "buyer", req, hgBuyHash(req))
	if !errors.Is(err, ErrHGStock) {
		t.Fatalf("库存不足应拒绝支付: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGBuyConcurrentReplayAfterStockExhaustionReturnsOriginalOrder(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	req := HGBuyRequest{ProductID: "PRD_1", RequestID: "buy-1", Quantity: 1, ReceiverName: "王明", ReceiverPhone: "13800000000", ReceiverAddress: "上海市收货地址"}
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGMallOrderByRequestSQL)).WithArgs("buyer", req.RequestID).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGMallBuyProductSQL)).WithArgs(req.ProductID).WillReturnRows(sqlmock.NewRows([]string{"product_id", "title", "merchant_id", "user_id", "sku_id", "sku_title", "amount", "available"}).AddRow(req.ProductID, "商品", "MER_1", "seller", "SKU_1", "标准版", 100, 0))
	mock.ExpectRollback()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGMallOrderByRequestSQL)).WithArgs("buyer", req.RequestID).WillReturnRows(sqlmock.NewRows([]string{"id", "order_id", "merchant_id", "user_id", "product_id", "product_title", "sku_title", "sku_id", "quantity", "unit_amount", "payable_amount", "trade_status", "shipping_status", "receiver_name", "receiver_phone", "receiver_address", "carrier", "tracking_number", "request_hash", "debit_id", "payment_id"}).AddRow(1, "MORD_1", "MER_1", "buyer", req.ProductID, "商品", "标准版", "SKU_1", 1, 100, 100, 2, 0, req.ReceiverName, req.ReceiverPhone, req.ReceiverAddress, "", "", hgBuyHash(req), 12, "MPAY_1"))
	order, err := HGNewOrderRepository(db).Buy(context.Background(), "buyer", req, hgBuyHash(req))
	if err != nil || order.OrderID != "MORD_1" {
		t.Fatalf("并发重放未返回已付订单: %+v, %v", order, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGBuyCommitsOrderStockPaymentAndCoinTogether(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	req := HGBuyRequest{ProductID: "PRD_1", RequestID: "buy-2", Quantity: 2, ReceiverName: "王明", ReceiverPhone: "13800000000", ReceiverAddress: "上海市收货地址"}
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGMallOrderByRequestSQL)).WithArgs("buyer", req.RequestID).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGMallBuyProductSQL)).WithArgs(req.ProductID).WillReturnRows(sqlmock.NewRows([]string{"product_id", "title", "merchant_id", "user_id", "sku_id", "sku_title", "amount", "available"}).AddRow("PRD_1", "商品", "MER_1", "seller", "SKU_1", "标准版", 50, 3))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGMallTakeStockSQL)).WithArgs(uint32(2), uint32(2), "SKU_1", uint32(2), uint32(2)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGMallTradeInsertSQL)).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.EnsureCoinWalletSQL)).WithArgs("buyer").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectCoinWalletForUpdateSQL)).WithArgs("buyer").WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(200))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertCoinRequestSQL)).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectCoinLotsForDebitSQL)).WillReturnRows(sqlmock.NewRows([]string{"id", "remaining_amount", "expires_at"}).AddRow(5, 200, nil))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.DebitCoinWalletSQL)).WithArgs(uint64(100), "buyer", uint64(100)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertCoinTransactionSQL)).WillReturnResult(sqlmock.NewResult(51, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.CompleteCoinRequestSQL)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.UpdateCoinLotRemainingSQL)).WithArgs(uint64(100), uint64(5)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertCoinAllocationSQL)).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertOutboxEventSQL)).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGMallItemInsertSQL)).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGMallPaymentInsertSQL)).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGMallFulfillmentInsertSQL)).WithArgs(sqlmock.AnyArg(), "MER_1", "buyer", "SKU_1", sqlmock.AnyArg(), uint32(2), uint64(51), req.ReceiverName, req.ReceiverPhone, req.ReceiverAddress).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	repo := HGNewOrderRepository(db)
	order, err := repo.Buy(context.Background(), "buyer", req, hgBuyHash(req))
	if err != nil || order.Amount != 100 || order.TradeStatus != 2 || order.Quantity != 2 {
		t.Fatalf("下单结果=%+v 错误=%v", order, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGShipRejectsRefundedOrderBeforeWritingTracking(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGMallSellerOrderForUpdateSQL)).WithArgs("MORD_1", "seller").WillReturnRows(sqlmock.NewRows([]string{"amount", "trade_status", "shipping_status", "user_id", "carrier", "tracking_number"}).AddRow(100, 7, 3, "buyer", "", ""))
	mock.ExpectRollback()
	err = HGNewOrderRepository(db).Ship(context.Background(), "seller", HGShipRequest{OrderID: "MORD_1", Carrier: "快递", TrackingNumber: "ABC123"})
	if !errors.Is(err, ErrHGState) {
		t.Fatalf("已退款订单不允许发货: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGRefundRejectsShippedOrderWithoutCrediting(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGMallRefundByRequestSQL)).WithArgs("buyer", "refund-1").WillReturnRows(sqlmock.NewRows([]string{"trade_order_id", "request_hash", "refund_status"}))
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGMallBuyerOrderForUpdateSQL)).WithArgs("MORD_1", "buyer").WillReturnRows(sqlmock.NewRows([]string{"sku_id", "quantity", "debit_transaction_id", "payment_id", "payable_amount", "trade_status", "shipping_status", "merchant_id"}).AddRow("SKU_1", 1, 23, "MPAY_1", 100, 3, 1, "MER_1"))
	mock.ExpectRollback()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGMallRefundByRequestSQL)).WithArgs("buyer", "refund-1").WillReturnRows(sqlmock.NewRows([]string{"trade_order_id", "request_hash", "refund_status"}))
	err = HGNewOrderRepository(db).Refund(context.Background(), "buyer", HGRefundRequest{OrderID: "MORD_1", RequestID: "refund-1"})
	if !errors.Is(err, ErrHGState) {
		t.Fatalf("已发货订单不允许自动退款: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGRefundRestoresStockAndOriginalDebitInOneCommit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	req := HGRefundRequest{OrderID: "MORD_1", RequestID: "refund-1"}
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGMallRefundByRequestSQL)).WithArgs("buyer", req.RequestID).WillReturnRows(sqlmock.NewRows([]string{"trade_order_id", "request_hash", "refund_status"}))
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGMallBuyerOrderForUpdateSQL)).WithArgs(req.OrderID, "buyer").WillReturnRows(sqlmock.NewRows([]string{"sku_id", "quantity", "debit_transaction_id", "payment_id", "payable_amount", "trade_status", "shipping_status", "merchant_id"}).AddRow("SKU_1", 2, 19, "MPAY_1", 100, 2, 0, "MER_1"))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGMallReturnStockSQL)).WithArgs(uint32(2), uint32(2), "SKU_1", uint32(2), uint32(2)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.EnsureCoinWalletSQL)).WithArgs("buyer").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectCoinWalletForUpdateSQL)).WithArgs("buyer").WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(50))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertCoinRequestSQL)).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectCoinDebitForRefundSQL)).WithArgs(uint64(19), "buyer").WillReturnRows(sqlmock.NewRows([]string{"id", "amount", "refunded"}).AddRow(19, 100, 0))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.CreditCoinWalletSQL)).WithArgs(uint64(100), "buyer", uint64(math.MaxInt64-100)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertCoinTransactionSQL)).WillReturnResult(sqlmock.NewResult(22, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.CompleteCoinRequestSQL)).WithArgs(uint64(22), uint64(150), "buyer", "mall:refund:MORD_1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertCoinLotSQL)).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertOutboxEventSQL)).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGMallRefundInsertSQL)).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGMallTradeRefundSQL)).WithArgs(req.OrderID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGMallFulfillmentRefundSQL)).WithArgs(req.OrderID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	err = HGNewOrderRepository(db).Refund(context.Background(), "buyer", req)
	if err != nil {
		t.Fatalf("退款失败: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGReceiveSettlesSellerOnlyAfterBuyerConfirms(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGMallBuyerOrderForUpdateSQL)).WithArgs("MORD_1", "buyer").WillReturnRows(sqlmock.NewRows([]string{"sku_id", "quantity", "debit_transaction_id", "payment_id", "payable_amount", "trade_status", "shipping_status", "merchant_id"}).AddRow("SKU_1", 1, 19, "MPAY_1", 100, 3, 1, "MER_1"))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGMallMerchantUserSQL)).WithArgs("MER_1").WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow("seller"))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.EnsureCoinWalletSQL)).WithArgs("seller").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectCoinWalletForUpdateSQL)).WithArgs("seller").WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(0))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertCoinRequestSQL)).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.CreditCoinWalletSQL)).WithArgs(uint64(100), "seller", uint64(math.MaxInt64-100)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertCoinTransactionSQL)).WillReturnResult(sqlmock.NewResult(22, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.CompleteCoinRequestSQL)).WithArgs(uint64(22), uint64(100), "seller", "mall:settle:MORD_1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertCoinLotSQL)).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertOutboxEventSQL)).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGMallReceiveSQL)).WithArgs("MORD_1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.HGMallTradeReceivedSQL)).WithArgs("MORD_1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := HGNewOrderRepository(db).Receive(context.Background(), "buyer", "MORD_1"); err != nil {
		t.Fatalf("买家确认收货结算失败: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGBuyValidatesAddressAndQuantityBeforeDatabase(t *testing.T) {
	req := HGBuyRequest{ProductID: "PRD_1", RequestID: "req-1", Quantity: 1, ReceiverName: "王明", ReceiverPhone: "13800000000", ReceiverAddress: "上海市"}
	service := HGNewService(nil, nil)
	for _, change := range []func(*HGBuyRequest){func(r *HGBuyRequest) { r.Quantity = 11 }, func(r *HGBuyRequest) { r.ReceiverAddress = "<script>" }, func(r *HGBuyRequest) { r.ReceiverPhone = "+123x567" }} {
		invalid := req
		change(&invalid)
		if _, err := service.HGBuy(context.Background(), "buyer", invalid); !errors.Is(err, ErrHGInvalid) {
			t.Errorf("无效请求应在查询前拒绝: %v", err)
		}
	}
}

func TestHGOrderLookupIsScopedToBuyer(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGMallOrderByOwnerSQL)).WithArgs("other-user", "MORD_1").WillReturnError(sql.ErrNoRows)
	_, err = HGNewOrderRepository(db).Order(context.Background(), "other-user", "MORD_1", false)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("跨用户读取订单应不可见: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
