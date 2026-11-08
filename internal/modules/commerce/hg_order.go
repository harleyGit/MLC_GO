package commerce

import (
	q "MLC_GO/internal/pkg/mysql/queries"
	util "MLC_GO/internal/pkg/utils"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"
	"time"
)

var (
	ErrHGStock              = errors.New("人民币库存不足或商品不可购买")
	ErrHGPaymentUnavailable = errors.New("人民币支付渠道尚未验收，订单和发货未开放")
)

// HGReserveRequest 是未来生产 CNY 结算协议；当前不注册 HTTP 写路由，不能用 debug 支付完成它。
type HGReserveRequest struct {
	ProductID    string `json:"productId"`    // 上架生产 CNY 商品 ID，不接受 M币或沙箱商品。
	SKUID        string `json:"skuId"`        // 当前公开内容版本的 SKU ID。
	Quantity     uint32 `json:"quantity"`     // 单笔 1-10 件。
	AddressID    string `json:"addressId"`    // 本人地址业务 ID，由服务端读取地址快照。
	ExpectedCent int64  `json:"expectedCent"` // 浏览器试算总额，人民币分；服务端重新计算并比较。
	RequestID    string `json:"requestId"`    // 本买家不可复用的创建幂等键，同键异正文冲突。
}

// HGOrderQuote 为待支付订单的只读服务端报价，库存只表示瞬时可售量，不保证已预留。
type HGOrderQuote struct {
	ProductID  string          `json:"productId"`  // 生产商品业务 ID。
	SKUID      string          `json:"skuId"`      // 已审核线上版本 SKU ID。
	MerchantID string          `json:"merchantId"` // 卖家业务 ID，用于禁止本人购买。
	Title      string          `json:"title"`      // 公开标题快照。
	SKUCode    string          `json:"skuCode"`    // 跨版本库存标识，客户端不能自定库存行。
	Specs      json.RawMessage `json:"specs"`      // 已审核规格 JSON 快照。
	Quantity   uint32          `json:"quantity"`   // 单笔购买数量 1-10 件。
	UnitCent   int64           `json:"unitCent"`   // 单价人民币分。
	TotalCent  int64           `json:"totalCent"`  // 当前报价人民币分，暂不含运费、优惠和币抵扣。
	Available  int64           `json:"available"`  // 查询时可售件数，可能并发变化。
}

// HGOrder 是生产 CNY 待支付订单及历史授权快照；买家列表和商家列表不填充私人地址字段。
type HGOrder struct {
	ID              uint64          `json:"-"`                         // 数据库内部列表游标。
	OrderID         string          `json:"orderId"`                   // 共用 trade_order 业务 ID，生产 CNY 独立前缀。
	UserID          string          `json:"userId"`                    // 可信买家业务 ID，仅本人及已支付本店订单可见。
	MerchantID      string          `json:"merchantId"`                // 下单时已核准商家 ID。
	ProductID       string          `json:"productId"`                 // 商品 ID 下单快照。
	SKUID           string          `json:"skuId"`                     // 下单时线上版本 SKU ID。
	SKUCode         string          `json:"skuCode"`                   // 库存关联编码快照。
	Title           string          `json:"title"`                     // 商品标题快照。
	Specs           json.RawMessage `json:"specs"`                     // 规格快照。
	Quantity        uint32          `json:"quantity"`                  // 购买件数，1-10。
	UnitCent        int64           `json:"unitCent"`                  // 人民币单价，整数分。
	TotalCent       int64           `json:"totalCent"`                 // 应付总额人民币分，不代表已支付。
	Status          uint8           `json:"status"`                    // 1待支付、2已支付、3已发货、4完成、5取消；无渠道时只能存在1或5。
	ExpireAt        time.Time       `json:"expireAt"`                  // 订单失效时间，UTC毫秒。
	AddressID       string          `json:"addressId,omitempty"`       // 仅本人或已支付本店详情返回地址来源 ID。
	ReceiverName    string          `json:"receiverName,omitempty"`    // 仅本人或已支付本店详情返回收件人。
	ReceiverPhone   string          `json:"receiverPhone,omitempty"`   // 仅授权详情返回收件手机，不进列表与日志。
	ReceiverAddress string          `json:"receiverAddress,omitempty"` // 仅授权详情返回完整地址快照。
	Carrier         string          `json:"carrier,omitempty"`         // 已发货时人工承运商名称。
	TrackingNumber  string          `json:"trackingNumber,omitempty"`  // 已发货时人工填写运单号。
}

// HGOrderPage 使用稳定主键倒序游标，不提供大表 count 或深 offset。
type HGOrderPage struct {
	List       []HGOrder `json:"list"`       // 本人或本店单页最多20单。
	NextCursor string    `json:"nextCursor"` // 仅有下一页时返回最后一个内部 ID。
	HasMore    bool      `json:"hasMore"`    // limit+1 判定是否仍有订单。
}

// HGStockAdjustment 是本人店铺人工盘点库存请求；建议库存不会自动成为可售库存。
type HGStockAdjustment struct {
	ProductID       string `json:"productId"`       // 本店已审核上架商品 ID。
	SKUCode         string `json:"skuCode"`         // 当前公开版本中的 SKU 编码。
	Delta           int64  `json:"delta"`           // 可售库存增减量，-100000 至 100000 且不为零。
	ExpectedVersion uint64 `json:"expectedVersion"` // 库存乐观锁版本，用于冲突重读。
	Reason          string `json:"reason"`          // 本次盘点原因或依据，1-256字符；审计保留，不写请求日志。
}

// HGStockSnapshot 是本店当前库存快照；调用方不得把建议库存当成可售库存。
type HGStockSnapshot struct {
	Available int64  `json:"available"` // 当前可售件数。
	Reserved  int64  `json:"reserved"`  // 待支付预占件数。
	Sold      int64  `json:"sold"`      // 已确认支付的件数。
	Version   uint64 `json:"version"`   // 盘点乐观锁版本。
}

// HGStock 仅本人已获批商家可读所属 SKU 的库存事实。
func (s *HGOrderStore) HGStock(ctx context.Context, user, product, sku string) (HGStockSnapshot, error) {
	if user == "" || !hgID(product) || !hgID(sku) {
		return HGStockSnapshot{}, ErrHGInvalid
	}
	var stock HGStockSnapshot
	err := s.db.QueryRowContext(ctx, q.HGCommerceStockReadSQL, user, product, sku).Scan(&stock.Available, &stock.Reserved, &stock.Sold, &stock.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return HGStockSnapshot{}, ErrHGForbidden
	}
	return stock, err
}

// HGShipRequest 仅供真实渠道支付确认后的商家人工发货；当前无支付渠道写入已支付事实。
type HGShipRequest struct {
	OrderID        string `json:"orderId"`        // 已支付生产 CNY 订单 ID。
	Carrier        string `json:"carrier"`        // 纯文本承运商，1-64字符。
	TrackingNumber string `json:"trackingNumber"` // 1-128位字母数字及点、横线、下划线。
}

// HGOrderStore 在同一 MySQL 事务内写统一 trade_order、CNY 订单与库存预占；不实现支付回调。
type HGOrderStore struct{ db *sql.DB }

// HGNewOrderStore 只装配共享 MySQL 池，库存订单写入口当前不注册生产 HTTP。
func HGNewOrderStore(db *sql.DB) *HGOrderStore { return &HGOrderStore{db: db} }

func hgReserveValid(r HGReserveRequest) bool {
	return hgID(r.ProductID) && hgID(r.SKUID) && hgID(r.AddressID) && hgID(r.RequestID) && r.Quantity >= 1 && r.Quantity <= 10 && r.ExpectedCent >= 1 && r.ExpectedCent <= 10000000000
}

func hgRequestHash(r HGReserveRequest) [32]byte {
	data, _ := json.Marshal(r)
	return sha256.Sum256(data)
}

func hgQuoteScan(row interface{ Scan(...any) error }, req HGReserveRequest) (HGOrderQuote, error) {
	result := HGOrderQuote{ProductID: req.ProductID, SKUID: req.SKUID, Quantity: req.Quantity}
	var specs []byte
	err := row.Scan(&result.MerchantID, &result.Title, &result.SKUCode, &specs, &result.UnitCent, &result.Available)
	if err != nil {
		return HGOrderQuote{}, err
	}
	result.Specs = specs
	if result.UnitCent <= 0 || result.UnitCent > 1000000000 || result.Available < 0 || int64(req.Quantity) > math.MaxInt64/result.UnitCent {
		return HGOrderQuote{}, ErrHGConflict
	}
	result.TotalCent = result.UnitCent * int64(req.Quantity)
	return result, nil
}

// HGPreview 只读公开 SKU 的服务端报价，调用方不能将报价当作已锁库存或已支付。
func (s *HGOrderStore) HGPreview(ctx context.Context, user string, r HGReserveRequest) (HGOrderQuote, error) {
	if user == "" || !hgReserveValid(r) {
		return HGOrderQuote{}, ErrHGInvalid
	}
	quote, err := hgQuoteScan(s.db.QueryRowContext(ctx, q.HGCommerceOrderQuoteSQL, r.SKUID, r.ProductID), r)
	if errors.Is(err, sql.ErrNoRows) {
		return HGOrderQuote{}, ErrHGStock
	}
	if err != nil {
		return HGOrderQuote{}, err
	}
	if quote.Available < int64(r.Quantity) {
		return quote, ErrHGStock
	}
	return quote, nil
}

// HGReserve 只供后续已验收支付渠道的受控结算入口调用；在渠道接入前不注册 HTTP。
// 商品行锁→库存条件更新→统一订单/明细→生产订单/预占同事务；失败回滚不遗留已扣库存。
func (s *HGOrderStore) HGReserve(ctx context.Context, user string, r HGReserveRequest) (HGOrder, error) {
	if user == "" || !hgReserveValid(r) {
		return HGOrder{}, ErrHGInvalid
	}
	hash := hgRequestHash(r)
	if old, err := s.hgReplay(ctx, user, r.RequestID, hash); err == nil {
		return old, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return HGOrder{}, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return HGOrder{}, err
	}
	defer tx.Rollback()
	var productID uint64
	if err = tx.QueryRowContext(ctx, q.HGCommerceOrderProductLockSQL, r.ProductID).Scan(&productID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return HGOrder{}, ErrHGStock
		}
		return HGOrder{}, err
	}
	// 另一个同键请求可能在等待商品锁期间提交；回滚本事务后再查询可避免重复占库存。
	var previousID string
	var previousHash []byte
	err = tx.QueryRowContext(ctx, q.HGCommerceOrderReplaySQL, user, r.RequestID).Scan(&previousID, &previousHash)
	if err == nil {
		_ = tx.Rollback()
		return s.hgReplay(ctx, user, r.RequestID, hash)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return HGOrder{}, err
	}
	quote, err := hgQuoteScan(tx.QueryRowContext(ctx, q.HGCommerceOrderQuoteSQL, r.SKUID, r.ProductID), r)
	if errors.Is(err, sql.ErrNoRows) {
		return HGOrder{}, ErrHGStock
	}
	if err != nil {
		return HGOrder{}, err
	}
	if quote.MerchantID == "" || quote.TotalCent != r.ExpectedCent {
		return HGOrder{}, ErrHGConflict
	}
	var owns int
	err = tx.QueryRowContext(ctx, q.HGCommerceOrderBuyerMerchantSQL, quote.MerchantID, user).Scan(&owns)
	if err == nil {
		return HGOrder{}, ErrHGForbidden
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return HGOrder{}, err
	}
	var name, phone, region, detail string
	err = tx.QueryRowContext(ctx, q.HGCommerceOrderAddressSQL, r.AddressID, user).Scan(&name, &phone, &region, &detail)
	if errors.Is(err, sql.ErrNoRows) {
		return HGOrder{}, ErrHGForbidden
	}
	if err != nil {
		return HGOrder{}, err
	}
	if !hgText(name, 64, true) || !hgText(region, 128, true) || !hgText(detail, 512, true) || len(phone) < 7 || len(phone) > 32 {
		return HGOrder{}, ErrHGInvalid
	}
	if err = hgExactlyOne(tx.ExecContext(ctx, q.HGCommerceStockReserveSQL, r.Quantity, r.Quantity, r.ProductID, quote.SKUCode, r.Quantity)); err != nil {
		if errors.Is(err, ErrHGConflict) {
			return HGOrder{}, ErrHGStock
		}
		return HGOrder{}, err
	}
	id := HGReserveID()
	expiry := time.Now().UTC().Add(15 * time.Minute)
	if _, err = tx.ExecContext(ctx, q.HGCommerceTradeCreateSQL, id, user, r.RequestID, hex.EncodeToString(hash[:]), quote.Title, quote.TotalCent, quote.TotalCent, expiry); err != nil {
		return s.hgReserveFailure(ctx, tx, user, r.RequestID, hash, err)
	}
	if _, err = tx.ExecContext(ctx, q.HGCommerceTradeItemSQL, util.GenerateBusinessID("COMITEM"), id, user, r.ProductID, r.SKUID, quote.Title, quote.SKUCode, r.Quantity, quote.UnitCent, quote.TotalCent); err != nil {
		return HGOrder{}, err
	}
	if _, err = tx.ExecContext(ctx, q.HGCommerceOrderCreateSQL, id, user, quote.MerchantID, r.ProductID, quote.SKUCode, r.SKUID, quote.Title, quote.Specs, r.Quantity, quote.UnitCent, quote.TotalCent, r.RequestID, hash[:], r.AddressID, name, phone, region+" "+detail, expiry); err != nil {
		return s.hgReserveFailure(ctx, tx, user, r.RequestID, hash, err)
	}
	if _, err = tx.ExecContext(ctx, q.HGCommerceReservationCreateSQL, util.GenerateBusinessID("COMRES"), id, r.ProductID, quote.SKUCode, r.Quantity); err != nil {
		return HGOrder{}, err
	}
	if err = tx.Commit(); err != nil {
		return HGOrder{}, err
	}
	return s.HGOrder(ctx, user, id)
}

func (s *HGOrderStore) hgReserveFailure(ctx context.Context, tx *sql.Tx, user, key string, hash [32]byte, failure error) (HGOrder, error) {
	_ = tx.Rollback()
	if old, err := s.hgReplay(ctx, user, key, hash); err == nil || errors.Is(err, ErrHGConflict) {
		return old, err
	}
	return HGOrder{}, hgDBError(failure)
}

func (s *HGOrderStore) hgReplay(ctx context.Context, user, key string, hash [32]byte) (HGOrder, error) {
	var id string
	var stored []byte
	err := s.db.QueryRowContext(ctx, q.HGCommerceOrderReplaySQL, user, key).Scan(&id, &stored)
	if err != nil {
		return HGOrder{}, err
	}
	if len(stored) != len(hash) || string(stored) != string(hash[:]) {
		return HGOrder{}, ErrHGConflict
	}
	return s.HGOrder(ctx, user, id)
}

func hgOrderScan(row interface{ Scan(...any) error }, withAddress bool) (HGOrder, error) {
	var order HGOrder
	var specs []byte
	fields := []any{&order.ID, &order.OrderID, &order.UserID, &order.MerchantID, &order.ProductID, &order.SKUID, &order.SKUCode, &order.Title, &specs, &order.Quantity, &order.UnitCent, &order.TotalCent, &order.Status, &order.ExpireAt}
	if withAddress {
		fields = append(fields, &order.AddressID, &order.ReceiverName, &order.ReceiverPhone, &order.ReceiverAddress, &order.Carrier, &order.TrackingNumber)
	}
	err := row.Scan(fields...)
	order.Specs = specs
	return order, err
}

// HGOrder 只读本人订单及收件快照，查无此订单时不暴露其他买家的存在性。
func (s *HGOrderStore) HGOrder(ctx context.Context, user, id string) (HGOrder, error) {
	if user == "" || !hgID(id) {
		return HGOrder{}, ErrHGInvalid
	}
	order, err := hgOrderScan(s.db.QueryRowContext(ctx, q.HGCommerceOrderOwnerSQL, id, user), true)
	if errors.Is(err, sql.ErrNoRows) {
		return HGOrder{}, ErrHGForbidden
	}
	return order, err
}

// HGOrders 按本人或已支付本店订单稳定游标分页；列表不查询收件个人信息。
func (s *HGOrderStore) HGOrders(ctx context.Context, user, cursor string, merchant bool) (HGOrderPage, error) {
	if user == "" {
		return HGOrderPage{}, ErrHGForbidden
	}
	id, err := hgCursor(cursor)
	if err != nil {
		return HGOrderPage{}, err
	}
	query := q.HGCommerceOrderBuyerPageSQL
	if merchant {
		query = q.HGCommerceOrderSellerPageSQL
	}
	rows, err := s.db.QueryContext(ctx, query, user, id)
	if err != nil {
		return HGOrderPage{}, err
	}
	defer rows.Close()
	items := make([]HGOrder, 0, 21)
	for rows.Next() {
		item, scanErr := hgOrderScan(rows, false)
		if scanErr != nil {
			return HGOrderPage{}, scanErr
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return HGOrderPage{}, err
	}
	page := HGOrderPage{List: items, HasMore: len(items) > 20}
	if page.HasMore {
		page.List = items[:20]
		page.NextCursor = strconv.FormatUint(page.List[19].ID, 10)
	}
	return page, nil
}

// HGAdjustStock 仅供 debug 受控盘点入口使用；同事务写不可变审计，不将 initial_stock 自动入库。
func (s *HGOrderStore) HGAdjustStock(ctx context.Context, user string, r HGStockAdjustment) error {
	if user == "" || !hgID(r.ProductID) || !hgID(r.SKUCode) || r.Delta == 0 || r.Delta > 100000 || r.Delta < -100000 || r.ExpectedVersion == 0 || !hgText(r.Reason, 256, true) || strings.TrimSpace(r.Reason) == "" {
		return ErrHGInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var version uint64
	var available int64
	if err = tx.QueryRowContext(ctx, q.HGCommerceStockOwnedLockSQL, user, r.ProductID, r.SKUCode).Scan(&version, &available); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrHGForbidden
		}
		return err
	}
	if version != r.ExpectedVersion {
		return ErrHGConflict
	}
	if err = hgExactlyOne(tx.ExecContext(ctx, q.HGCommerceStockAdjustSQL, r.Delta, r.ProductID, r.SKUCode, version, r.Delta, r.Delta)); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, q.HGCommerceStockAuditSQL, util.GenerateBusinessID("COMADJ"), user, r.ProductID, r.SKUCode, r.Delta, available, available+r.Delta, version, r.Reason); err != nil {
		return err
	}
	return tx.Commit()
}

// HGCancel 仅取消本人尚未支付的订单；管理员过期批次按同一锁序调用私有入口。
func (s *HGOrderStore) HGCancel(ctx context.Context, user, id string) error {
	if user == "" || !hgID(id) {
		return ErrHGInvalid
	}
	return s.hgClose(ctx, user, id, false)
}

func (s *HGOrderStore) hgClose(ctx context.Context, user, id string, expired bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var owner, product, sku string
	var quantity uint32
	var status uint8
	var expiry time.Time
	err = tx.QueryRowContext(ctx, q.HGCommerceOrderCloseLockSQL, id).Scan(&owner, &product, &sku, &quantity, &status, &expiry)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrHGForbidden
	}
	if err != nil {
		return err
	}
	if !expired && owner != user {
		return ErrHGForbidden
	}
	if status == 5 {
		return nil
	}
	if status != 1 || (expired && expiry.After(time.Now().UTC())) {
		return ErrHGConflict
	}
	if err = hgExactlyOne(tx.ExecContext(ctx, q.HGCommerceReservationReleaseSQL, id)); err != nil {
		return err
	}
	if err = hgExactlyOne(tx.ExecContext(ctx, q.HGCommerceStockReleaseSQL, quantity, quantity, product, sku, quantity)); err != nil {
		return err
	}
	if err = hgExactlyOne(tx.ExecContext(ctx, q.HGCommerceOrderCloseSQL, id)); err != nil {
		return err
	}
	if err = hgExactlyOne(tx.ExecContext(ctx, q.HGCommerceTradeCloseSQL, id)); err != nil {
		return err
	}
	return tx.Commit()
}

// HGExpiryBatch 每次只处理最多20个到期订单；未来支付回调必须与关闭竞争同一订单行锁。
func (s *HGOrderStore) HGExpiryBatch(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, q.HGCommerceOrderExpiredPageSQL)
	if err != nil {
		return err
	}
	ids := make([]string, 0, 20)
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err = s.hgClose(ctx, "", id, true); err != nil && !errors.Is(err, ErrHGConflict) {
			return err
		}
	}
	return nil
}

// HGShip 仅未来真实渠道验签入账后可调用；必须同时存在支付单成功事实，不信任前端状态。
func (s *HGOrderStore) HGShip(ctx context.Context, user string, r HGShipRequest) error {
	if user == "" || !hgID(r.OrderID) || !hgText(r.Carrier, 64, true) || len(r.TrackingNumber) < 1 || len(r.TrackingNumber) > 128 {
		return ErrHGInvalid
	}
	for _, c := range r.TrackingNumber {
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '.' || c == '-' || c == '_') {
			return ErrHGInvalid
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var status uint8
	err = tx.QueryRowContext(ctx, q.HGCommerceShipLockSQL, user, r.OrderID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrHGForbidden
	}
	if err != nil {
		return err
	}
	if status != 2 {
		return ErrHGConflict
	}
	if err = hgExactlyOne(tx.ExecContext(ctx, q.HGCommerceShipUpdateSQL, r.Carrier, r.TrackingNumber, r.OrderID)); err != nil {
		return err
	}
	if err = hgExactlyOne(tx.ExecContext(ctx, q.HGCommerceShipTradeSQL, r.OrderID)); err != nil {
		return err
	}
	return tx.Commit()
}

// HGReserveID 前缀独立于 M币和模拟商城，便于对账时识别正式 CNY 未支付订单。
func HGReserveID() string { return util.GenerateBusinessID("COMORD") }
