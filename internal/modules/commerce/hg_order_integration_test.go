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
	if err = db.QueryRowContext(ctx, q.HGGetCrawlerMigrationVersionSQL).Scan(&migration, &dirty); err != nil || migration != 66 || dirty {
		t.Fatalf("隔离库版本不符合约束：version=%d dirty=%t err=%v", migration, dirty, err)
	}
	seller := util.GenerateBusinessID("COMTESTSELL")
	buyer := util.GenerateBusinessID("COMTESTBUY")
	merchant := util.GenerateBusinessID("COMTESTMER")
	address := util.GenerateBusinessID("COMTESTADDR")
	mediaURL := "https://example.invalid/" + util.GenerateBusinessID("COMTESTIMG") + ".jpg"
	createKey := util.GenerateBusinessID("COMTESTDRAFT")
	var category int64
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
		if category > 0 {
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
	result, err := db.ExecContext(ctx, q.HGCommerceFixtureCategorySQL, "隔离测试普通类目")
	if err != nil {
		t.Fatal(err)
	}
	category, err = result.LastInsertId()
	if err != nil {
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
	draft := HGDraftInput{CategoryID: fmt.Sprint(category), ProductType: "PHYSICAL", Title: "隔离测试商品", ImageURLs: []string{mediaURL}, SKUs: []HGSKUInput{{SKUCode: "BLACK", Specs: map[string]string{}, PriceCent: 100, InitialStock: 10}}}
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
	if writer.Code != http.StatusServiceUnavailable {
		t.Fatalf("未验收渠道不允许付款：%d", writer.Code)
	}
	writer = httptest.NewRecorder()
	handler.HGOrderShipment(writer, request(http.MethodPost, "/merchant/orders/ship", seller, HGShipRequest{OrderID: orders[2].OrderID, Carrier: "快递", TrackingNumber: "TEST-123"}))
	if writer.Code != http.StatusServiceUnavailable {
		t.Fatalf("未验收渠道不允许发货：%d", writer.Code)
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
}
