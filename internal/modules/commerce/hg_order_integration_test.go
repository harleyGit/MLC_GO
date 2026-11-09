package commerce

import (
	userjwt "MLC_GO/internal/modules/user/middleware"
	usersvc "MLC_GO/internal/modules/user/service"
	cfg "MLC_GO/internal/pkg/config"
	q "MLC_GO/internal/pkg/mysql/queries"
	util "MLC_GO/internal/pkg/utils"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

type hgCommerceTestReviewer struct{}

func (hgCommerceTestReviewer) HasAssetPermission(context.Context, string, string) (bool, error) {
	return true, nil
}

// TestHGCommerceDebugLifecycle 仅在显式隔离 debug 授权下运行；测试身份随机且清理范围限制在本次 ID。
func TestHGCommerceDebugLifecycle(t *testing.T) {
	if os.Getenv("MLC_COMMERCE_INTEGRATION") != "1" {
		t.Skip("未显式授权生产 CNY 商品的隔离 debug 集成验收")
	}
	if os.Getenv("SERVER_ENV") != "debug" {
		t.Fatal("仅允许显式 debug 测试进程")
	}
	t.Setenv("MLC_CONFIG_DIR", "../../../config")
	t.Setenv("MLC_COMMERCE_DEBUG_TRADING", "true")
	t.Setenv("MLC_COMMERCE_SIMULATED_WEBHOOK_KEY", "integration-only-simulated-channel-v1-key-unique")
	if err := cfg.LoadConfig("debug"); err != nil {
		t.Fatal(err)
	}
	config, err := cfg.GetMySQLConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.Host != "127.0.0.1" && config.Host != "localhost" {
		t.Fatal("拒绝非本机数据库")
	}
	db, err := sql.Open("mysql", (&mysql.Config{User: config.User, Passwd: config.Password, Net: "tcp", Addr: config.Host + ":" + config.Port, DBName: config.Database, ParseTime: true, Loc: time.UTC}).FormatDSN())
	if err != nil {
		t.Fatal("无法装配本机 debug 数据库")
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(12)
	ctx, stop := context.WithTimeout(context.Background(), 45*time.Second)
	defer stop()
	var migration uint64
	var dirty bool
	if err = db.QueryRowContext(ctx, q.HGGetCrawlerMigrationVersionSQL).Scan(&migration, &dirty); err != nil || migration != 69 || dirty {
		t.Fatalf("隔离库版本不符合约束：version=%d dirty=%t err=%v", migration, dirty, err)
	}
	seller := util.GenerateBusinessID("COMTESTSELL")
	buyer := util.GenerateBusinessID("COMTESTBUY")
	merchant := util.GenerateBusinessID("COMTESTMER")
	address := util.GenerateBusinessID("COMTESTADDR")
	mediaURL := "https://example.invalid/" + util.GenerateBusinessID("COMTESTIMG") + ".jpg"
	createKey := util.GenerateBusinessID("COMTESTDRAFT")
	category := util.GenerateBusinessID("COMTESTCAT")
	var product string
	// 先注册清理：即使创建后的响应丢失，仍按唯一幂等键找到本次商品并精确删除。
	t.Cleanup(func() {
		cleanCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		clean := func(statement string, args ...any) {
			if _, cleanErr := db.ExecContext(cleanCtx, statement, args...); cleanErr != nil {
				t.Errorf("隔离夹具清理失败：%v", cleanErr)
			}
		}
		if product == "" {
			_ = db.QueryRowContext(cleanCtx, q.HGCommerceFixtureFindDraftSQL, merchant, createKey).Scan(&product)
		}
		rows, readErr := db.QueryContext(cleanCtx, q.HGCommerceFixtureOrderIDsSQL, buyer)
		orderIDs := make([]string, 0, 10)
		if readErr == nil {
			for rows.Next() {
				var id string
				if scanErr := rows.Scan(&id); scanErr == nil {
					orderIDs = append(orderIDs, id)
				}
			}
			rows.Close()
		} else {
			t.Errorf("读取测试订单清单失败：%v", readErr)
		}
		for _, id := range orderIDs {
			clean(q.HGCommerceFixtureRemoveSimEventSQL, id)
			clean(q.HGCommerceFixtureRemoveSimRefundSQL, id, buyer, "SIMREF_"+id)
			clean(q.HGCommerceFixtureRemoveSimPaymentSQL, id, buyer)
			clean(q.HGCommerceFixtureRemoveReservationSQL, id)
			clean(q.HGCommerceFixtureRemoveItemSQL, id, buyer)
			clean(q.HGCommerceFixtureRemoveOrderSQL, id, buyer)
			clean(q.HGCommerceFixtureRemoveTradeSQL, id, buyer)
		}
		clean(q.HGCommerceFixtureRemoveAddressSQL, address, buyer)
		if product != "" {
			clean(q.HGCommerceFixtureRemoveStockAuditSQL, product, seller)
			clean(q.HGCommerceFixtureRemoveStockSQL, product)
			clean(q.HGCommerceFixtureRemoveSKUSQL, product)
			clean(q.HGCommerceFixtureRemoveVersionSQL, product)
			clean(q.HGCommerceFixtureRemovePublishKeySQL, merchant, createKey)
			clean(q.HGCommerceFixtureRemoveProductSQL, product)
		}
		clean(q.HGCommerceFixtureRemoveMediaSQL, seller, mediaURL)
		if category != "" {
			clean(q.HGCommerceFixtureRemoveCategorySQL, category)
		}
		clean(q.HGCommerceFixtureRemoveMerchantSQL, merchant)
		clean(q.HGCommerceFixtureRemoveUserSQL, buyer)
	})
	if _, err = db.ExecContext(ctx, q.HGCommerceFixtureUserSQL, buyer, buyer, "not-a-login-hash", "test-only-salt"); err != nil {
		t.Fatalf("创建隔离买家失败：%v", err)
	}
	if _, err = db.ExecContext(ctx, q.HGCommerceFixtureMerchantSQL, merchant, seller, "隔离测试店铺"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, q.HGCommerceFixtureCategorySQL, category, "隔离测试普通类目"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, q.HGCommerceFixtureMediaSQL, seller, mediaURL, "integration-only"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, q.HGCommerceFixtureAddressSQL, address, buyer, "隔离收件人", "13800000000", "测试地区", "110000", "110100", "隔离测试地址"); err != nil {
		t.Fatal(err)
	}
	service := HGNewService(HGNewRepository(db), hgCommerceTestReviewer{})
	handler := HGNewHandler(service, nil).HGWithOrderStore(HGNewOrderStore(db))
	request := func(method, path, user string, payload any) *http.Request {
		var body []byte
		if payload != nil {
			body, _ = json.Marshal(payload)
		}
		httpRequest := httptest.NewRequest(method, path, bytes.NewReader(body))
		return httpRequest.WithContext(context.WithValue(httpRequest.Context(), userjwt.UserIDKey, &usersvc.HGClaims{UserID: user}))
	}
	draft := HGDraftInput{CategoryID: category, ProductType: "PHYSICAL", Title: "隔离测试商品", ImageURLs: []string{mediaURL}, SKUs: []HGSKUInput{{SKUCode: "BLACK", Specs: map[string]string{}, PriceCent: 100, InitialStock: 10}}}
	create := request(http.MethodPost, "/merchant/products", seller, draft)
	create.Header.Set("Idempotency-Key", createKey)
	writer := httptest.NewRecorder()
	handler.HGCreateDraft(writer, create)
	if writer.Code != http.StatusOK {
		t.Fatalf("草稿 HTTP 状态=%d", writer.Code)
	}
	// 独立按创建幂等键读取商品，避免依赖响应包装格式。
	if err = db.QueryRowContext(ctx, q.HGCommerceFixtureFindDraftSQL, merchant, createKey).Scan(&product); err != nil {
		t.Fatal(err)
	}
	for _, action := range []struct {
		fn         http.HandlerFunc
		path, user string
		body       any
	}{
		{handler.HGSubmit, "/merchant/products/submit", seller, map[string]any{"productId": product, "expectedVersion": uint64(1)}},
		{handler.HGReview, "/admin/products/decision", buyer, map[string]any{"productId": product, "approve": true, "reason": ""}},
		{handler.HGPublish, "/merchant/products/publish", seller, map[string]any{"productId": product, "expectedVersion": uint64(3)}},
	} {
		writer = httptest.NewRecorder()
		action.fn(writer, request(http.MethodPost, action.path, action.user, action.body))
		if writer.Code != http.StatusOK {
			t.Fatalf("商品流程 %s 状态=%d", action.path, writer.Code)
		}
	}
	writer = httptest.NewRecorder()
	handler.HGDetail(writer, request(http.MethodGet, "/products/detail?productId="+product, buyer, nil))
	if writer.Code != http.StatusOK || !strings.Contains(writer.Body.String(), product) {
		t.Fatalf("公开商品详情未按已审版本返回：%d", writer.Code)
	}
	var sku string
	if err = db.QueryRowContext(ctx, q.HGCommerceFixtureFindSKUSQL, product).Scan(&sku); err != nil {
		t.Fatal(err)
	}
	// 审核只有零库存；本次显式人工盘点在夹具中确认10件，不将 initial_stock 当成可售事实。
	var available, reserved, sold int64
	if err = db.QueryRowContext(ctx, q.HGCommerceFixtureStockReadSQL, product).Scan(&available, &reserved, &sold); err != nil || available != 0 {
		t.Fatalf("审核后库存须为零：%d %v", available, err)
	}
	writer = httptest.NewRecorder()
	handler.HGInventoryAdjust(writer, request(http.MethodPost, "/merchant/inventory/adjust", seller, HGStockAdjustment{ProductID: product, SKUCode: "BLACK", Delta: 10, ExpectedVersion: 1, Reason: "隔离测试实物盘点10件"}))
	if writer.Code != http.StatusOK {
		t.Fatalf("debug 本人盘点 HTTP 状态=%d", writer.Code)
	}
	var auditCount int
	if err = db.QueryRowContext(ctx, q.HGCommerceFixtureStockAuditCountSQL, product, seller).Scan(&auditCount); err != nil || auditCount != 1 {
		t.Fatalf("盘点与审计必须同事务持久化：count=%d err=%v", auditCount, err)
	}
	store := HGNewOrderStore(db)
	const contenders = 100
	var group sync.WaitGroup
	var mutex sync.Mutex
	orders := make([]HGOrder, 0, 10)
	unexpected := make([]error, 0)
	for i := 0; i < contenders; i++ {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			r := HGReserveRequest{ProductID: product, SKUID: sku, Quantity: 1, AddressID: address, ExpectedCent: 100, RequestID: fmt.Sprintf("reserve-%d-%s", index, buyer[len(buyer)-8:])}
			order, orderErr := store.HGReserve(ctx, buyer, r)
			mutex.Lock()
			defer mutex.Unlock()
			if orderErr == nil {
				orders = append(orders, order)
			} else if !errors.Is(orderErr, ErrHGStock) {
				unexpected = append(unexpected, orderErr)
			}
		}(i)
	}
	group.Wait()
	if len(unexpected) > 0 {
		t.Fatalf("并发预占出现意外错误（首个）：%v", unexpected[0])
	}
	if len(orders) != 10 {
		t.Fatalf("100次争抢10件，成功=%d，期望10", len(orders))
	}
	if err = db.QueryRowContext(ctx, q.HGCommerceFixtureStockReadSQL, product).Scan(&available, &reserved, &sold); err != nil || available != 0 || reserved != 10 || sold != 0 {
		t.Fatalf("预占守恒：%d %d %d err=%v", available, reserved, sold, err)
	}
	if err = store.HGCancel(ctx, buyer, orders[0].OrderID); err != nil {
		t.Fatal(err)
	}
	if err = store.HGCancel(ctx, buyer, orders[0].OrderID); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, q.HGCommerceFixtureStockReadSQL, product).Scan(&available, &reserved, &sold); err != nil || available != 1 || reserved != 9 || sold != 0 {
		t.Fatalf("重复取消不应重复释放库存：%d %d %d err=%v", available, reserved, sold, err)
	}
	if _, err = db.ExecContext(ctx, q.HGCommerceFixtureExpireOrderSQL, orders[1].OrderID, buyer); err != nil {
		t.Fatal(err)
	}
	if err = store.HGExpiryBatch(ctx); err != nil {
		t.Fatal(err)
	}
	if err = store.HGExpiryBatch(ctx); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, q.HGCommerceFixtureStockReadSQL, product).Scan(&available, &reserved, &sold); err != nil || available != 2 || reserved != 8 || sold != 0 {
		t.Fatalf("过期批次不得重复释放库存：%d %d %d err=%v", available, reserved, sold, err)
	}
	if _, err = store.HGOrder(ctx, seller, orders[1].OrderID); !errors.Is(err, ErrHGForbidden) {
		t.Fatalf("跨用户读取待支付收件地址应拒绝：%v", err)
	}
	if err = store.HGShip(ctx, seller, HGShipRequest{OrderID: orders[1].OrderID, Carrier: "快递", TrackingNumber: "TEST-123"}); !errors.Is(err, ErrHGForbidden) {
		t.Fatalf("未验签支付不得发货：%v", err)
	}
	writer = httptest.NewRecorder()
	handler.HGOrderCreate(writer, request(http.MethodPost, "/orders", buyer, HGReserveRequest{ProductID: product, SKUID: sku, Quantity: 1, AddressID: address, ExpectedCent: 100, RequestID: "http-debug-" + buyer[len(buyer)-8:]}))
	if writer.Code != http.StatusOK {
		t.Fatalf("debug 订单预占 HTTP 状态=%d", writer.Code)
	}
	writer = httptest.NewRecorder()
	handler.HGOrderPayment(writer, request(http.MethodPost, "/orders/payment", buyer, map[string]any{"orderId": orders[2].OrderID}))
	if writer.Code != http.StatusOK {
		t.Fatalf("debug 模拟支付应只创建待回调尝试：%d", writer.Code)
	}
	writer = httptest.NewRecorder()
	handler.HGOrderShipment(writer, request(http.MethodPost, "/merchant/orders/ship", seller, HGShipRequest{OrderID: orders[2].OrderID, Carrier: "快递", TrackingNumber: "TEST-123"}))
	if writer.Code != http.StatusConflict {
		t.Fatalf("未到账不得发货：%d", writer.Code)
	}
	var httpOrderID string
	if err = db.QueryRowContext(ctx, q.HGCommerceFixtureOrderByRequestSQL, buyer, "http-debug-"+buyer[len(buyer)-8:]).Scan(&httpOrderID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, q.HGCommerceFixtureExpireOrderSQL, httpOrderID, buyer); err != nil {
		t.Fatal(err)
	}
	worker := hgStartExpiryWorker(store, 20*time.Millisecond)
	t.Cleanup(worker.Close)
	wait, waitCancel := context.WithTimeout(ctx, 3*time.Second)
	defer waitCancel()
	poll := time.NewTicker(25 * time.Millisecond)
	defer poll.Stop()
	for {
		item, readErr := store.HGOrder(wait, buyer, httpOrderID)
		if readErr != nil {
			t.Fatalf("查询自动过期订单失败：%v", readErr)
		}
		if item.Status == 5 {
			break
		}
		select {
		case <-wait.Done():
			t.Fatal("有界 worker 未能自动释放到期预占")
		case <-poll.C:
		}
	}
	worker.Close()
	if err = db.QueryRowContext(ctx, q.HGCommerceFixtureStockReadSQL, product).Scan(&available, &reserved, &sold); err != nil || available != 2 || reserved != 8 || sold != 0 {
		t.Fatalf("自动释放库存不守恒：%d %d %d err=%v", available, reserved, sold, err)
	}
	// 每单使用唯一模拟流水及事件；管理员回调经原始字节 HMAC 验证，不能用买家提交模拟付款成功。
	callback := func(orderID, paymentID, eventID string, sign bool) int {
		event := HGSimulatedEvent{EventID: eventID, PaymentID: paymentID, OrderID: orderID, ExternalPaymentID: "SIMEXT_" + eventID, Currency: "CNY", AmountCent: 100, Status: "SUCCESS"}
		raw, _ := json.Marshal(event)
		req := httptest.NewRequest(http.MethodPost, "/admin/simulated-payment/callback", bytes.NewReader(raw))
		req = req.WithContext(context.WithValue(req.Context(), userjwt.UserIDKey, &usersvc.HGClaims{UserID: buyer}))
		signature := "00"
		if sign {
			signature = HGSimulatedSignature([]byte(os.Getenv("MLC_COMMERCE_SIMULATED_WEBHOOK_KEY")), raw)
		}
		req.Header.Set("X-Simulated-Signature", signature)
		w := httptest.NewRecorder()
		handler.HGSimulatedCallback(w, req)
		return w.Code
	}
	paid := orders[2]
	pay, err := store.HGStartSimulatedPayment(ctx, buyer, paid.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	eventID := util.GenerateBusinessID("SIMEVT")
	if status := callback(paid.OrderID, pay.PaymentID, eventID, false); status != http.StatusForbidden {
		t.Fatalf("错误签名不得入账：%d", status)
	}
	if status := callback(paid.OrderID, pay.PaymentID, eventID, true); status != http.StatusOK {
		t.Fatalf("验签后模拟入账：%d", status)
	}
	if status := callback(paid.OrderID, pay.PaymentID, eventID, true); status != http.StatusOK {
		t.Fatalf("同事件重放应幂等：%d", status)
	}
	// 同一事件改写金额或流水即使持有有效测试签名也不能覆盖既有事实。
	tampered := HGSimulatedEvent{EventID: eventID, PaymentID: pay.PaymentID, OrderID: paid.OrderID, ExternalPaymentID: "SIMEXT_" + eventID, Currency: "CNY", AmountCent: 101, Status: "SUCCESS"}
	rawTampered, _ := json.Marshal(tampered)
	if _, err = store.HGApplySimulatedEvent(ctx, []byte(os.Getenv("MLC_COMMERCE_SIMULATED_WEBHOOK_KEY")), rawTampered, HGSimulatedSignature([]byte(os.Getenv("MLC_COMMERCE_SIMULATED_WEBHOOK_KEY")), rawTampered)); !errors.Is(err, ErrHGConflict) {
		t.Fatalf("已使用事件号不得变更金额：%v", err)
	}
	if status := callback(paid.OrderID, pay.PaymentID, util.GenerateBusinessID("SIMEVT"), true); status != http.StatusConflict {
		t.Fatalf("已入账订单不得接受第二次收款：%d", status)
	}
	if _, err = store.HGSellerOrder(ctx, buyer, paid.OrderID); !errors.Is(err, ErrHGForbidden) {
		t.Fatalf("跨店收件详情必须拒绝：%v", err)
	}
	w := httptest.NewRecorder()
	handler.HGOrderShipment(w, request(http.MethodPost, "/merchant/orders/ship", seller, HGShipRequest{OrderID: paid.OrderID, Carrier: "快递", TrackingNumber: "TEST-123"}))
	if w.Code != http.StatusOK {
		t.Fatalf("验签到账本店发货失败：%d", w.Code)
	}
	shipOrder, err := store.HGOrder(ctx, buyer, paid.OrderID)
	if err != nil || shipOrder.Status != 3 {
		t.Fatalf("模拟订单发货状态异常: %d %v", shipOrder.Status, err)
	}
	if err = store.HGRefundSimulated(ctx, buyer, paid.OrderID); !errors.Is(err, ErrHGConflict) {
		t.Fatalf("已发货订单不允许模拟未发货退款：%v", err)
	}
	refundOrder := orders[3]
	refundPay, err := store.HGStartSimulatedPayment(ctx, buyer, refundOrder.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	if status := callback(refundOrder.OrderID, refundPay.PaymentID, util.GenerateBusinessID("SIMEVT"), true); status != http.StatusOK {
		t.Fatalf("退款前付款失败：%d", status)
	}
	w = httptest.NewRecorder()
	handler.HGOrderRefund(w, request(http.MethodPost, "/orders/refund", buyer, map[string]string{"orderId": refundOrder.OrderID}))
	if w.Code != http.StatusOK {
		t.Fatalf("模拟退款失败：%d", w.Code)
	}
	if err = store.HGRefundSimulated(ctx, buyer, refundOrder.OrderID); err != nil {
		t.Fatalf("模拟退款重复请求须幂等：%v", err)
	}
	if err = store.HGShipSimulated(ctx, seller, HGShipRequest{OrderID: refundOrder.OrderID, Carrier: "快递", TrackingNumber: "TEST-123"}); !errors.Is(err, ErrHGConflict) {
		t.Fatalf("退款后发货必须拒绝：%v", err)
	}
	lateOrder := orders[4]
	latePay, err := store.HGStartSimulatedPayment(ctx, buyer, lateOrder.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.HGCancel(ctx, buyer, lateOrder.OrderID); err != nil {
		t.Fatal(err)
	}
	if status := callback(lateOrder.OrderID, latePay.PaymentID, util.GenerateBusinessID("SIMEVT"), true); status != http.StatusOK {
		t.Fatalf("取消后迟到收款须退款：%d", status)
	}
	lateStatus, err := store.HGReconcileSimulated(ctx, lateOrder.OrderID)
	if err != nil || lateStatus.Drift || lateStatus.RefundStatus != 2 {
		t.Fatalf("迟到回调模拟退款对账失败：%+v %v", lateStatus, err)
	}
	raceOrder := orders[5]
	racePay, err := store.HGStartSimulatedPayment(ctx, buyer, raceOrder.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, q.HGCommerceFixtureExpireOrderSQL, raceOrder.OrderID, buyer); err != nil {
		t.Fatal(err)
	}
	var race sync.WaitGroup
	race.Add(2)
	var callbackCode int
	var expiryErr error
	go func() {
		defer race.Done()
		callbackCode = callback(raceOrder.OrderID, racePay.PaymentID, util.GenerateBusinessID("SIMEVT"), true)
	}()
	go func() { defer race.Done(); expiryErr = store.HGExpiryBatch(ctx) }()
	race.Wait()
	if callbackCode != http.StatusOK || expiryErr != nil {
		t.Fatalf("过期/迟到回调竞争失败：callback=%d expiry=%v", callbackCode, expiryErr)
	}
	raceResult, err := store.HGReconcileSimulated(ctx, raceOrder.OrderID)
	if err != nil || raceResult.Drift || raceResult.RefundStatus != 2 {
		t.Fatalf("过期付款退款对账异常：%+v %v", raceResult, err)
	}
	// 已付订单的退款与发货在同一订单行锁上竞争，只允许一个终态。
	shippingRace := orders[6]
	shippingPay, err := store.HGStartSimulatedPayment(ctx, buyer, shippingRace.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	if status := callback(shippingRace.OrderID, shippingPay.PaymentID, util.GenerateBusinessID("SIMEVT"), true); status != http.StatusOK {
		t.Fatalf("竞争前模拟入账失败：%d", status)
	}
	var settle sync.WaitGroup
	settle.Add(2)
	var refundErr, shipErr error
	go func() { defer settle.Done(); refundErr = store.HGRefundSimulated(ctx, buyer, shippingRace.OrderID) }()
	go func() {
		defer settle.Done()
		shipErr = store.HGShipSimulated(ctx, seller, HGShipRequest{OrderID: shippingRace.OrderID, Carrier: "快递", TrackingNumber: "TEST-456"})
	}()
	settle.Wait()
	if (refundErr == nil) == (shipErr == nil) {
		t.Fatalf("退款/发货必须恰好一个成功：refund=%v ship=%v", refundErr, shipErr)
	}
	if refundErr != nil && !errors.Is(refundErr, ErrHGConflict) {
		t.Fatalf("退款竞争错误：%v", refundErr)
	}
	if shipErr != nil && !errors.Is(shipErr, ErrHGConflict) && !errors.Is(shipErr, ErrHGForbidden) {
		t.Fatalf("发货竞争错误：%v", shipErr)
	}
	raced, err := store.HGReconcileSimulated(ctx, shippingRace.OrderID)
	if err != nil || raced.Drift {
		t.Fatalf("竞争后账本漂移：%+v %v", raced, err)
	}
	if err = db.QueryRowContext(ctx, q.HGCommerceFixtureStockReadSQL, product).Scan(&available, &reserved, &sold); err != nil || available < 0 || reserved < 0 || sold < 0 {
		t.Fatalf("模拟付款退款竞争库存出现负值：%d %d %d %v", available, reserved, sold, err)
	}
	if available+reserved+sold != 10 {
		t.Fatalf("隔离盘点10件库存守恒失败：available=%d reserved=%d sold=%d", available, reserved, sold)
	}
	// 浏览器管理员无需获取签名密钥：经后端显式模拟收款入口调用同一验签状态机。
	manual := orders[7]
	if _, err = store.HGStartSimulatedPayment(ctx, buyer, manual.OrderID); err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	handler.HGSimulatedSettle(w, request(http.MethodPost, "/admin/simulated-payment/settle", buyer, map[string]string{"orderId": manual.OrderID}))
	if w.Code != http.StatusOK {
		t.Fatalf("管理员模拟验签入账失败：%d", w.Code)
	}
	manualStatus, err := store.HGReconcileSimulated(ctx, manual.OrderID)
	if err != nil || manualStatus.Drift || manualStatus.OrderStatus != 2 {
		t.Fatalf("管理员模拟付款账本异常：%+v %v", manualStatus, err)
	}
	// 模拟付款成功后，管理员重试同一单不可重演支付事实。
	w = httptest.NewRecorder()
	handler.HGSimulatedSettle(w, request(http.MethodPost, "/admin/simulated-payment/settle", buyer, map[string]string{"orderId": manual.OrderID}))
	if w.Code != http.StatusConflict {
		t.Fatalf("重复模拟入账应冲突：%d", w.Code)
	}
}
