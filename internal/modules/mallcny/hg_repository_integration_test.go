package mallcny

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
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

// TestHGCNYSandboxLifecycle 只在明确授权的本机隔离 debug 库运行；夹具均使用唯一业务 ID 且退出时精确清理。
func TestHGCNYSandboxLifecycle(t *testing.T) {
	if os.Getenv("MLC_MALL_CNY_INTEGRATION") != "1" {
		t.Skip("须显式授权本机隔离 debug 集成库")
	}
	if os.Getenv("SERVER_ENV") != "debug" {
		t.Fatal("只能在显式 debug 进程运行")
	}
	t.Setenv("MLC_CONFIG_DIR", "../../../config")
	t.Setenv("MLC_MALL_CNY_SANDBOX", "true")
	if err := cfg.LoadConfig("debug"); err != nil {
		t.Fatal(err)
	}
	config, err := cfg.GetMySQLConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.Host != "127.0.0.1" && config.Host != "localhost" {
		t.Fatal("禁止连接非本机数据库")
	}
	db, err := sql.Open("mysql", (&mysql.Config{User: config.User, Passwd: config.Password, Net: "tcp", Addr: config.Host + ":" + config.Port, DBName: config.Database, ParseTime: true, Loc: time.UTC}).FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	// Cleanup 后关闭连接，避免提前关闭连接池导致夹具残留。
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var version uint64
	var dirty bool
	if err = db.QueryRowContext(ctx, q.HGGetCrawlerMigrationVersionSQL).Scan(&version, &dirty); err != nil || version != 62 || dirty {
		t.Fatalf("隔离库版本不安全: version=%d dirty=%t err=%v", version, dirty, err)
	}
	repo := HGNewRepository(db)
	seller := util.GenerateBusinessID("CNYTESTSELL")
	merchant := util.GenerateBusinessID("CNYTESTMER")
	if _, err = db.ExecContext(ctx, q.HGCNYFixtureMerchantInsertSQL, merchant, seller, "沙箱集成测试店"); err != nil {
		t.Fatal(err)
	}
	var product HGProduct
	var coupon HGCoupon
	orders := make([]string, 0, 4)
	groups := make([]string, 0, 2)
	claimUser := ""
	t.Cleanup(func() {
		// 精确删除本次夹具，先清理订单子表；每个 DELETE 由生成的唯一 ID 限定。
		cleanCtx, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		clean := func(statement string, args ...any) {
			if _, cleanErr := db.ExecContext(cleanCtx, statement, args...); cleanErr != nil {
				t.Errorf("人民币沙箱夹具清理失败: %v", cleanErr)
			}
		}
		for _, id := range orders {
			clean(q.HGCNYFixtureDeleteRefundSQL, id)
			clean(q.HGCNYFixtureDeletePaymentSQL, id)
		}
		if coupon.CouponID != "" && claimUser != "" {
			clean(q.HGCNYFixtureDeleteUserCouponSQL, claimUser, coupon.CouponID)
		}
		for _, id := range orders {
			clean(q.HGCNYFixtureDeleteOrderSQL, id)
		}
		for _, id := range groups {
			clean(q.HGCNYFixtureDeleteGroupSQL, id)
		}
		if coupon.CouponID != "" {
			clean(q.HGCNYFixtureDeleteCouponSQL, coupon.CouponID)
		}
		if product.ProductID != "" {
			clean(q.HGCNYFixtureDeleteInventorySQL, product.SKUID)
			clean(q.HGCNYFixtureDeleteProductSQL, product.ProductID)
		}
		clean(q.HGCNYFixtureDeleteMerchantSQL, merchant, seller)
	})
	price := int64(800)
	product, err = repo.HGPublish(ctx, seller, HGCreateProduct{Title: "测试商品", SKUTitle: "单规格", PriceFen: 1000, GroupPriceFen: &price, GroupMembers: 2, Stock: 5})
	if err != nil {
		t.Fatal(err)
	}
	buyer1, buyer2 := util.GenerateBusinessID("CNYBUY"), util.GenerateBusinessID("CNYBUY")
	// 使用真实处理器与 HTTP JSON 响应协议核对 UI 需要的人民币分字段，且不绕过本人订单权限。
	handler := HGNewHandler(repo)
	productRequest := httptest.NewRequest(http.MethodGet, "/cny/products/detail?productId="+product.ProductID, nil)
	productWriter := httptest.NewRecorder()
	handler.HGDetail(productWriter, productRequest)
	var productReply struct {
		Result HGProduct `json:"result"`
	}
	if productWriter.Code != http.StatusOK || json.Unmarshal(productWriter.Body.Bytes(), &productReply) != nil || productReply.Result.PriceFen != 1000 {
		t.Fatalf("人民币商品接口字段不匹配: status=%d", productWriter.Code)
	}
	coupon, err = repo.HGCreateCoupon(ctx, seller, HGCreateCoupon{ProductID: product.ProductID, ThresholdFen: 1000, DiscountFen: 100, Quantity: 1, ExpiresAt: time.Now().UTC().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	claimUser = buyer1
	if err = repo.HGClaimCoupon(ctx, buyer1, product.ProductID, coupon.CouponID); err != nil {
		t.Fatal(err)
	}
	if err = repo.HGClaimCoupon(ctx, buyer1, product.ProductID, coupon.CouponID); !errors.Is(err, ErrHGConflict) {
		t.Fatalf("重复领券应冲突: %v", err)
	}
	makeRequest := func(user, mode, group, couponID string) HGOrderRequest {
		return HGOrderRequest{ProductID: product.ProductID, SKUID: product.SKUID, Quantity: 1, PurchaseType: mode, GroupID: group, CouponID: couponID, Address: "隔离测试地址", RequestID: util.GenerateBusinessID("CNYREQ")}
	}
	buy := func(user string, request HGOrderRequest) HGOrder {
		t.Helper()
		quote, e := repo.HGPreview(ctx, user, request)
		if e != nil {
			t.Fatal(e)
		}
		request.ExpectedPayableFen = quote.PayableFen
		order, e := repo.HGBuy(ctx, user, request)
		if e != nil {
			t.Fatal(e)
		}
		orders = append(orders, order.OrderID)
		return order
	}
	singleReq := makeRequest(buyer1, "single", "", coupon.CouponID)
	singleReq.ExpectedPayableFen = 900
	hgRequest := func(path string, payload HGOrderRequest) *http.Request {
		t.Helper()
		body, encodeErr := json.Marshal(payload)
		if encodeErr != nil {
			t.Fatal(encodeErr)
		}
		request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
		return request.WithContext(context.WithValue(request.Context(), userjwt.UserIDKey, &usersvc.HGClaims{UserID: buyer1}))
	}
	previewWriter := httptest.NewRecorder()
	handler.HGPreview(previewWriter, hgRequest("/cny/orders/preview", singleReq))
	var previewReply struct {
		Result HGQuote `json:"result"`
	}
	if previewWriter.Code != http.StatusOK || json.Unmarshal(previewWriter.Body.Bytes(), &previewReply) != nil || previewReply.Result.PayableFen != 900 {
		t.Fatalf("试算接口金额/字段不匹配: status=%d", previewWriter.Code)
	}
	buyWriter := httptest.NewRecorder()
	handler.HGBuy(buyWriter, hgRequest("/cny/orders", singleReq))
	var buyReply struct {
		Result HGOrder `json:"result"`
	}
	if buyWriter.Code != http.StatusOK || json.Unmarshal(buyWriter.Body.Bytes(), &buyReply) != nil || buyReply.Result.OrderID == "" || buyReply.Result.PayableFen != 900 {
		t.Fatalf("下单接口金额/字段不匹配: status=%d", buyWriter.Code)
	}
	single := buyReply.Result
	orders = append(orders, single.OrderID)
	otherRequest := httptest.NewRequest(http.MethodGet, "/cny/orders/detail?orderId="+single.OrderID, nil)
	otherRequest = otherRequest.WithContext(context.WithValue(otherRequest.Context(), userjwt.UserIDKey, &usersvc.HGClaims{UserID: buyer2}))
	otherWriter := httptest.NewRecorder()
	handler.HGOrder(otherWriter, otherRequest)
	if otherWriter.Code != http.StatusNotFound {
		t.Fatalf("跨用户读取订单未拒绝: status=%d", otherWriter.Code)
	}
	replay, err := repo.HGBuy(ctx, buyer1, singleReq)
	if err != nil || replay.OrderID != single.OrderID {
		t.Fatalf("幂等重放失败: %v", err)
	}
	conflictReq := singleReq
	conflictReq.Address = "另一个地址"
	if _, err = repo.HGBuy(ctx, buyer1, conflictReq); !errors.Is(err, ErrHGConflict) {
		t.Fatalf("同键不同地址应冲突: %v", err)
	}
	cancelledSingle, err := repo.HGCancel(ctx, buyer1, single.OrderID)
	if err != nil || cancelledSingle.Status != 3 {
		t.Fatalf("待付款取消失败: %v %+v", err, cancelledSingle)
	}
	var couponStatus uint8
	if err = db.QueryRowContext(ctx, q.HGCNYFixtureCouponStatusSQL, buyer1, coupon.CouponID).Scan(&couponStatus); err != nil || couponStatus != 0 {
		t.Fatalf("取消未释放已预占券: %v status=%d", err, couponStatus)
	}
	singleReq.RequestID = util.GenerateBusinessID("CNYREQ")
	single, err = repo.HGBuy(ctx, buyer1, singleReq)
	if err != nil {
		t.Fatal(err)
	}
	orders = append(orders, single.OrderID)
	hgAction := func(path string, action http.HandlerFunc, want uint8) HGOrder {
		t.Helper()
		body, encodeErr := json.Marshal(struct {
			OrderID string `json:"orderId"`
		}{single.OrderID})
		if encodeErr != nil {
			t.Fatal(encodeErr)
		}
		request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
		request = request.WithContext(context.WithValue(request.Context(), userjwt.UserIDKey, &usersvc.HGClaims{UserID: buyer1}))
		writer := httptest.NewRecorder()
		action(writer, request)
		var response struct {
			Result HGOrder `json:"result"`
		}
		if writer.Code != http.StatusOK || json.Unmarshal(writer.Body.Bytes(), &response) != nil || response.Result.Status != want {
			t.Fatalf("订单动作接口不匹配: path=%s status=%d", path, writer.Code)
		}
		return response.Result
	}
	paid := hgAction("/cny/orders/pay", handler.HGDebugPay, 1)
	if paid.PayableFen != 900 {
		t.Fatalf("模拟支付金额不符: %d", paid.PayableFen)
	}
	refunded := hgAction("/cny/orders/refund", handler.HGDebugRefund, 4)
	if refunded.PayableFen != 900 {
		t.Fatalf("模拟退款金额不符: %d", refunded.PayableFen)
	}
	if _, err = repo.HGDebugRefund(ctx, buyer1, single.OrderID); err != nil {
		t.Fatalf("重复退款不幂等: %v", err)
	}
	leader := buy(buyer1, makeRequest(buyer1, "group_create", "", ""))
	groups = append(groups, leader.GroupID)
	if _, err = repo.HGDebugPay(ctx, buyer1, leader.OrderID); err != nil {
		t.Fatal(err)
	}
	joined := buy(buyer2, makeRequest(buyer2, "group_join", leader.GroupID, ""))
	if _, err = db.ExecContext(ctx, q.HGCNYFixtureExpireGroupSQL, leader.GroupID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, q.HGCNYFixtureExpireOrderSQL, joined.OrderID); err != nil {
		t.Fatal(err)
	}
	if err = repo.HGExpiryBatch(ctx); err != nil {
		t.Fatal(err)
	}
	failed, err := repo.HGOrder(ctx, buyer1, leader.OrderID)
	if err != nil || failed.Status != 4 {
		t.Fatalf("过期团长未模拟退款: %v %+v", err, failed)
	}
	cancelled, err := repo.HGOrder(ctx, buyer2, joined.OrderID)
	if err != nil || cancelled.Status != 3 {
		t.Fatalf("过期参团单未取消: %v %+v", err, cancelled)
	}
	group, err := repo.HGGroup(ctx, leader.GroupID)
	if err != nil || group.Status != 5 {
		t.Fatalf("到期拼团未结清: %v %+v", err, group)
	}
	leader2 := buy(buyer1, makeRequest(buyer1, "group_create", "", ""))
	groups = append(groups, leader2.GroupID)
	if _, err = repo.HGDebugPay(ctx, buyer1, leader2.OrderID); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.HGDebugRefund(ctx, buyer1, leader2.OrderID); !errors.Is(err, ErrHGConflict) {
		t.Fatalf("未失败的团不得退款: %v", err)
	}
	joined2 := buy(buyer2, makeRequest(buyer2, "group_join", leader2.GroupID, ""))
	otherBuyer := util.GenerateBusinessID("CNYBUY")
	fullRequest := makeRequest(otherBuyer, "group_join", leader2.GroupID, "")
	fullRequest.ExpectedPayableFen = 800
	if _, err = repo.HGBuy(ctx, otherBuyer, fullRequest); !errors.Is(err, ErrHGConflict) {
		t.Fatalf("满员团不应再占席: %v", err)
	}
	if _, err = repo.HGDebugPay(ctx, buyer2, joined2.OrderID); err != nil {
		t.Fatal(err)
	}
	ready, err := repo.HGOrder(ctx, buyer1, leader2.OrderID)
	if err != nil || ready.Status != 1 {
		t.Fatalf("成团未更新团长订单: %v %+v", err, ready)
	}
	// 同一团仅剩一席时并发两个不同买家，最多允许一个订单预占；随后到期恢复库存。
	competitiveLeader := buy(buyer1, makeRequest(buyer1, "group_create", "", ""))
	groups = append(groups, competitiveLeader.GroupID)
	if _, err = repo.HGDebugPay(ctx, buyer1, competitiveLeader.OrderID); err != nil {
		t.Fatal(err)
	}
	type hgJoinResult struct {
		order HGOrder
		user  string
		err   error
	}
	results := make(chan hgJoinResult, 2)
	var workers sync.WaitGroup
	for _, user := range []string{buyer2, otherBuyer} {
		workers.Add(1)
		go func(user string) {
			defer workers.Done()
			request := makeRequest(user, "group_join", competitiveLeader.GroupID, "")
			request.ExpectedPayableFen = 800
			order, joinErr := repo.HGBuy(ctx, user, request)
			results <- hgJoinResult{order: order, user: user, err: joinErr}
		}(user)
	}
	workers.Wait()
	close(results)
	var winner hgJoinResult
	success, conflicts := 0, 0
	for result := range results {
		if result.err == nil {
			winner = result
			success++
			orders = append(orders, result.order.OrderID)
		} else if errors.Is(result.err, ErrHGConflict) {
			conflicts++
		} else {
			t.Fatalf("并发预占发生非预期错误: %v", result.err)
		}
	}
	if success != 1 || conflicts != 1 {
		t.Fatalf("同一团仅剩一席时获得 %d 个席位、%d 个冲突", success, conflicts)
	}
	if _, err = db.ExecContext(ctx, q.HGCNYFixtureExpireGroupSQL, competitiveLeader.GroupID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, q.HGCNYFixtureExpireOrderSQL, winner.order.OrderID); err != nil {
		t.Fatal(err)
	}
	if err = repo.HGExpiryBatch(ctx); err != nil {
		t.Fatal(err)
	}
	var available, reserved, sold uint32
	if err = db.QueryRowContext(ctx, q.HGCNYFixtureInventorySQL, product.SKUID).Scan(&available, &reserved, &sold); err != nil || available != 3 || reserved != 0 || sold != 2 {
		t.Fatalf("库存守恒失败: %v available=%d reserved=%d sold=%d", err, available, reserved, sold)
	}
}
