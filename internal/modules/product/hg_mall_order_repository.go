package product

import (
	CoinRepositoryPackage "MLC_GO/internal/modules/coin/repository"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	UtilsPackage "MLC_GO/internal/pkg/utils"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"math"
	"strconv"
	"time"

	"github.com/go-sql-driver/mysql"
)

// HGOrderRepository 复用商品、订单、库存、coin 与 Outbox 所在的同一 MySQL 连接池。
type HGOrderRepository struct {
	db   *sql.DB                             // 已初始化的权威 MySQL 池；不得连接副本或独立事务。
	coin *CoinRepositoryPackage.HGRepository // 事务内资产与 lot/Outbox 写入器，不自行提交。
}

// HGNewOrderRepository 只装配依赖，不执行迁移或对外部系统写入。
func HGNewOrderRepository(db *sql.DB) *HGOrderRepository {
	return &HGOrderRepository{db: db, coin: CoinRepositoryPackage.NewHGRepository(db, "mlc.domain.events")}
}

func hgOrderScan(row interface{ Scan(...any) error }) (HGOrder, error) {
	var order HGOrder
	err := row.Scan(&order.ID, &order.OrderID, &order.MerchantID, &order.UserID, &order.ProductID, &order.Title, &order.SKUTitle, &order.SKUID, &order.Quantity, &order.UnitAmount, &order.Amount, &order.TradeStatus, &order.ShippingStatus, &order.ReceiverName, &order.ReceiverPhone, &order.ReceiverAddress, &order.Carrier, &order.TrackingNumber, &order.Hash, &order.DebitID, &order.PaymentID)
	return order, err
}

func hgPendingScan(row interface{ Scan(...any) error }) (HGOrder, error) {
	var order HGOrder
	var expires time.Time
	err := row.Scan(&order.ID, &order.OrderID, &order.MerchantID, &order.UserID, &order.ProductID, &order.Title, &order.SKUTitle, &order.SKUID, &order.Quantity, &order.UnitAmount, &order.Amount, &order.TradeStatus, &order.ReceiverName, &order.ReceiverPhone, &order.ReceiverAddress, &order.Hash, &expires)
	if err == nil {
		order.ExpiresAt = expires.UTC().Format(time.RFC3339Nano)
	}
	return order, err
}

func (r *HGOrderRepository) PendingOrders(ctx context.Context, user string, cursor uint64) ([]HGOrder, error) {
	rows, err := r.db.QueryContext(ctx, SQLQueriesPackage.HGMallPendingOrdersSQL, user, strconv.FormatUint(cursor, 10), 21)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]HGOrder, 0, 21)
	for rows.Next() {
		order, err := hgPendingScan(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, order)
	}
	return items, rows.Err()
}

func (r *HGOrderRepository) Order(ctx context.Context, user, orderID string, merchant bool) (HGOrder, error) {
	query := SQLQueriesPackage.HGMallOrderByOwnerSQL
	if merchant {
		query = SQLQueriesPackage.HGMallOrderByMerchantSQL
	}
	return hgOrderScan(r.db.QueryRowContext(ctx, query, user, orderID))
}

func (r *HGOrderRepository) Orders(ctx context.Context, user string, merchant bool, cursor uint64) ([]HGOrder, error) {
	query := SQLQueriesPackage.HGMallBuyerOrdersSQL
	if merchant {
		query = SQLQueriesPackage.HGMallSellerOrdersSQL
	}
	rows, err := r.db.QueryContext(ctx, query, user, strconv.FormatUint(cursor, 10), 21)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]HGOrder, 0, 21)
	for rows.Next() {
		order, err := hgOrderScan(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, order)
	}
	return items, rows.Err()
}

func (r *HGOrderRepository) hgReplayBuy(ctx context.Context, user, requestID, hash string) (HGOrder, error) {
	var orderID, savedHash string
	if err := r.db.QueryRowContext(ctx, SQLQueriesPackage.HGMallBuyReplayKeySQL, user, requestID).Scan(&orderID, &savedHash); err != nil {
		return HGOrder{}, err
	}
	if savedHash != hash {
		return HGOrder{}, ErrHGConflict
	}
	order, err := hgOrderScan(r.db.QueryRowContext(ctx, SQLQueriesPackage.HGMallOrderByOwnerSQL, user, orderID))
	if errors.Is(err, sql.ErrNoRows) {
		order, err = hgPendingScan(r.db.QueryRowContext(ctx, SQLQueriesPackage.HGMallPendingByOwnerSQL, user, orderID))
	}
	if errors.Is(err, sql.ErrNoRows) || (err == nil && (order.OrderID != orderID || order.Hash != savedHash)) {
		// 已存在幂等键却缺少实物订单快照，绝不可继续支付并覆盖其他订单类型。
		return HGOrder{}, ErrHGConflict
	}
	return order, err
}

// 并发重放可能在最初点查后、等待商品行锁期间才完成提交；库存用尽也应优先返回已提交的原订单。
func (r *HGOrderRepository) hgReplayAfterBuyFailure(ctx context.Context, tx *sql.Tx, user string, req HGBuyRequest, hash string, failure error) (HGOrder, error) {
	_ = tx.Rollback()
	previous, err := r.hgReplayBuy(ctx, user, req.RequestID, hash)
	if errors.Is(err, sql.ErrNoRows) {
		return HGOrder{}, failure
	}
	return previous, err
}

// Buy 先锁商品库存，在同一事务预留15分钟库存并写订单及收货快照，不触碰钱包。
// 结果未知时凭买家和 requestId 重放，不得重新占用库存。
func (r *HGOrderRepository) Buy(ctx context.Context, user string, req HGBuyRequest, hash string) (HGOrder, error) {
	previous, err := r.hgReplayBuy(ctx, user, req.RequestID, hash)
	if !errors.Is(err, sql.ErrNoRows) {
		return previous, err
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return HGOrder{}, err
	}
	defer tx.Rollback()
	var productID, title, merchantID, seller, skuID, skuTitle string
	var unit int64
	var available uint32
	err = tx.QueryRowContext(ctx, SQLQueriesPackage.HGMallBuyProductSQL, req.ProductID).Scan(&productID, &title, &merchantID, &seller, &skuID, &skuTitle, &unit, &available)
	if errors.Is(err, sql.ErrNoRows) {
		return r.hgReplayAfterBuyFailure(ctx, tx, user, req, hash, ErrHGStock)
	}
	if err != nil {
		return HGOrder{}, err
	}
	if seller == user {
		return HGOrder{}, ErrHGForbidden
	}
	if unit < 1 || unit > int64(CoinRepositoryPackage.HGMaxMallMutationAmount) || req.Quantity > available || unit > math.MaxInt64/int64(req.Quantity) || unit*int64(req.Quantity) > int64(CoinRepositoryPackage.HGMaxMallMutationAmount) {
		return r.hgReplayAfterBuyFailure(ctx, tx, user, req, hash, ErrHGStock)
	}
	amount := unit * int64(req.Quantity)
	orderID := UtilsPackage.GenerateBusinessID("MORD")
	stockResult, err := tx.ExecContext(ctx, SQLQueriesPackage.HGMallReserveStockSQL, req.Quantity, skuID, req.Quantity)
	if err != nil {
		return HGOrder{}, err
	}
	if err = hgOne(stockResult, nil); err != nil {
		if errors.Is(err, ErrHGConflict) {
			return r.hgReplayAfterBuyFailure(ctx, tx, user, req, hash, ErrHGStock)
		}
		return HGOrder{}, err
	}
	_, err = tx.ExecContext(ctx, SQLQueriesPackage.HGMallTradeInsertSQL, orderID, user, req.RequestID, hash, title, amount, amount)
	if err != nil {
		var duplicate *mysql.MySQLError
		if errors.As(err, &duplicate) && duplicate.Number == 1062 {
			_ = tx.Rollback()
			previous, replayErr := r.hgReplayBuy(ctx, user, req.RequestID, hash)
			if errors.Is(replayErr, sql.ErrNoRows) {
				replayErr = ErrHGConflict
			}
			return previous, replayErr
		}
		return HGOrder{}, err
	}
	itemID := UtilsPackage.GenerateBusinessID("MITM")
	if _, err = tx.ExecContext(ctx, SQLQueriesPackage.HGMallItemInsertSQL, itemID, orderID, user, productID, skuID, title, skuTitle, req.Quantity, unit, amount); err != nil {
		return HGOrder{}, err
	}
	if err = hgOne(tx.ExecContext(ctx, SQLQueriesPackage.HGMallPendingInsertSQL, orderID, user, merchantID, skuID, req.Quantity, req.ReceiverName, req.ReceiverPhone, req.ReceiverAddress, orderID, user)); err != nil {
		return HGOrder{}, err
	}
	if err = tx.Commit(); err != nil {
		return HGOrder{}, err
	}
	return hgPendingScan(r.db.QueryRowContext(ctx, SQLQueriesPackage.HGMallPendingByOwnerSQL, user, orderID))
}

// Pay 锁本人预留订单后一次提交扣币、支付事实、履约快照与订单状态；成功重放不二次扣币。
func (r *HGOrderRepository) Pay(ctx context.Context, user, orderID string) (HGOrder, error) {
	paid, err := r.Order(ctx, user, orderID, false)
	if err == nil {
		return paid, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return HGOrder{}, err
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return HGOrder{}, err
	}
	defer tx.Rollback()
	var merchant, sku, name, phone, address, hash string
	var status int8
	var amount int64
	var quantity uint32
	var beforeExpire bool
	err = tx.QueryRowContext(ctx, SQLQueriesPackage.HGMallPendingForPaymentSQL, orderID, user).Scan(&status, &amount, &hash, &beforeExpire, &merchant, &sku, &quantity, &name, &phone, &address)
	if errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		paid, err = r.Order(ctx, user, orderID, false)
		if err == nil {
			return paid, nil
		}
		if errors.Is(err, sql.ErrNoRows) {
			return HGOrder{}, ErrHGState
		}
		return HGOrder{}, err
	}
	if err != nil {
		return HGOrder{}, err
	}
	if status != 1 || !beforeExpire || amount < 1 || amount > int64(CoinRepositoryPackage.HGMaxMallMutationAmount) {
		_ = tx.Rollback()
		if status == 1 && !beforeExpire {
			if err := r.HGExpireOrder(ctx, orderID); err != nil {
				return HGOrder{}, err
			}
		}
		return HGOrder{}, ErrHGState
	}
	if err = hgOne(tx.ExecContext(ctx, SQLQueriesPackage.HGMallCountPaidStockSQL, quantity, sku, quantity)); err != nil {
		return HGOrder{}, err
	}
	coinResult, err := r.coin.HGMarketTx(ctx, tx, "debit", user, orderID, uint64(amount), 0)
	if errors.Is(err, CoinRepositoryPackage.ErrHGInsufficientBalance) || errors.Is(err, CoinRepositoryPackage.ErrHGBusinessLimit) {
		return HGOrder{}, ErrHGFunds
	}
	if err != nil {
		return HGOrder{}, err
	}
	paymentID := UtilsPackage.GenerateBusinessID("MPAY")
	if _, err = tx.ExecContext(ctx, SQLQueriesPackage.HGMallPaymentInsertSQL, paymentID, orderID, user, "mall:"+orderID, hash, amount); err != nil {
		return HGOrder{}, err
	}
	if _, err = tx.ExecContext(ctx, SQLQueriesPackage.HGMallFulfillmentInsertSQL, orderID, merchant, user, sku, paymentID, quantity, coinResult.TransactionID, name, phone, address); err != nil {
		return HGOrder{}, err
	}
	if err = hgOne(tx.ExecContext(ctx, SQLQueriesPackage.HGMallTradePaidSQL, orderID, user)); err != nil {
		if errors.Is(err, ErrHGConflict) {
			return HGOrder{}, ErrHGState
		}
		return HGOrder{}, err
	}
	if err = hgOne(tx.ExecContext(ctx, SQLQueriesPackage.HGMallPendingDeleteSQL, orderID)); err != nil {
		return HGOrder{}, err
	}
	if err = tx.Commit(); err != nil {
		return HGOrder{}, err
	}
	return r.Order(ctx, user, orderID, false)
}

// HGExpireOrder 逐单短事务锁定过期预留，只释放一次库存并保留取消订单的本人快照。
func (r *HGOrderRepository) HGExpireOrder(ctx context.Context, orderID string) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var status int8
	var sku string
	var qty uint32
	var reserved bool
	err = tx.QueryRowContext(ctx, SQLQueriesPackage.HGMallExpiredForUpdateSQL, orderID).Scan(&status, &sku, &qty, &reserved)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if !reserved {
		return nil
	}
	if status != 1 || qty == 0 || qty > 10 {
		return ErrHGState
	}
	if err = hgOne(tx.ExecContext(ctx, SQLQueriesPackage.HGMallReleaseReservationSQL, qty, sku, qty)); err != nil {
		return err
	}
	if err = hgOne(tx.ExecContext(ctx, SQLQueriesPackage.HGMallTradeCancelSQL, orderID)); err != nil {
		return err
	}
	if err = hgOne(tx.ExecContext(ctx, SQLQueriesPackage.HGMallPendingReleaseSQL, orderID)); err != nil {
		return err
	}
	return tx.Commit()
}

// HGExpireBatch 每轮最多读取100条过期业务键，再逐单短事务处理；失败留待下一轮重试。
func (r *HGOrderRepository) HGExpireBatch(ctx context.Context) error {
	rows, err := r.db.QueryContext(ctx, SQLQueriesPackage.HGMallExpiredReservationsSQL, 100)
	if err != nil {
		return err
	}
	ids := make([]string, 0, 100)
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			break
		}
		ids = append(ids, id)
	}
	if err == nil {
		err = rows.Err()
	}
	_ = rows.Close()
	if err != nil {
		return err
	}
	var failures error
	for _, id := range ids {
		if ctx.Err() != nil {
			return errors.Join(failures, ctx.Err())
		}
		if err := r.HGExpireOrder(ctx, id); err != nil {
			failures = errors.Join(failures, err)
		}
	}
	return failures
}

func (r *HGOrderRepository) Ship(ctx context.Context, seller string, req HGShipRequest) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var amount int64
	var trade, shipping int8
	var buyer, carrier, tracking string
	if err = tx.QueryRowContext(ctx, SQLQueriesPackage.HGMallSellerOrderForUpdateSQL, req.OrderID, seller).Scan(&amount, &trade, &shipping, &buyer, &carrier, &tracking); err != nil {
		return err
	}
	if trade == 3 && shipping == 1 && carrier == req.Carrier && tracking == req.TrackingNumber {
		return nil // 首次提交结果未知时同一运单可重放，不覆盖已保存的物流事实。
	}
	if trade != 2 || shipping != 0 {
		return ErrHGState
	}
	if err = hgOne(tx.ExecContext(ctx, SQLQueriesPackage.HGMallShipSQL, req.Carrier, req.TrackingNumber, req.OrderID)); err != nil {
		return err
	}
	if err = hgOne(tx.ExecContext(ctx, SQLQueriesPackage.HGMallTradeShippedSQL, req.OrderID)); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *HGOrderRepository) Receive(ctx context.Context, buyer, orderID string) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var sku, payment, merchantID string
	var quantity uint32
	var debit uint64
	var amount int64
	var trade, shipping int8
	if err = tx.QueryRowContext(ctx, SQLQueriesPackage.HGMallBuyerOrderForUpdateSQL, orderID, buyer).Scan(&sku, &quantity, &debit, &payment, &amount, &trade, &shipping, &merchantID); err != nil {
		return err
	}
	if trade == 4 && shipping == 2 {
		return nil // 已结算订单重试确认收货不能重复给卖家入账。
	}
	if trade != 3 || shipping != 1 || amount < 1 || amount > int64(CoinRepositoryPackage.HGMaxMallMutationAmount) {
		return ErrHGState
	}
	var seller string
	if err = tx.QueryRowContext(ctx, SQLQueriesPackage.HGMallMerchantUserSQL, merchantID).Scan(&seller); err != nil {
		return err
	}
	if _, err = r.coin.HGMarketTx(ctx, tx, "settle", seller, orderID, uint64(amount), 0); err != nil {
		return err
	}
	if err = hgOne(tx.ExecContext(ctx, SQLQueriesPackage.HGMallReceiveSQL, orderID)); err != nil {
		return err
	}
	if err = hgOne(tx.ExecContext(ctx, SQLQueriesPackage.HGMallTradeReceivedSQL, orderID)); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *HGOrderRepository) hgReplayRefund(ctx context.Context, buyer string, req HGRefundRequest) (bool, error) {
	var id, hash string
	var status int8
	err := r.db.QueryRowContext(ctx, SQLQueriesPackage.HGMallRefundByRequestSQL, buyer, req.RequestID).Scan(&id, &hash, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	sum := sha256.Sum256([]byte(req.OrderID))
	if id != req.OrderID || hash != hex.EncodeToString(sum[:]) || status != 2 {
		return false, ErrHGConflict
	}
	return true, nil
}

// Refund 仅在未发货时将库存、原扣款的全额退款与订单状态一次提交；同订单并发发货由行锁互斥。
func (r *HGOrderRepository) Refund(ctx context.Context, buyer string, req HGRefundRequest) error {
	if replay, err := r.hgReplayRefund(ctx, buyer, req); err != nil || replay {
		return err
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var sku, payment, merchantID string
	var quantity uint32
	var debit uint64
	var amount int64
	var trade, shipping int8
	if err = tx.QueryRowContext(ctx, SQLQueriesPackage.HGMallBuyerOrderForUpdateSQL, req.OrderID, buyer).Scan(&sku, &quantity, &debit, &payment, &amount, &trade, &shipping, &merchantID); err != nil {
		return err
	}
	if trade != 2 || shipping != 0 {
		_ = tx.Rollback()
		if replay, replayErr := r.hgReplayRefund(ctx, buyer, req); replayErr != nil || replay {
			return replayErr
		}
		return ErrHGState
	}
	if amount < 1 || amount > int64(CoinRepositoryPackage.HGMaxMallMutationAmount) {
		return ErrHGState
	}
	if err = hgOne(tx.ExecContext(ctx, SQLQueriesPackage.HGMallReturnStockSQL, quantity, quantity, sku, quantity, quantity)); err != nil {
		return err
	}
	if _, err = r.coin.HGMarketTx(ctx, tx, "refund", buyer, req.OrderID, uint64(amount), debit); err != nil {
		return err
	}
	sum := sha256.Sum256([]byte(req.OrderID))
	if _, err = tx.ExecContext(ctx, SQLQueriesPackage.HGMallRefundInsertSQL, UtilsPackage.GenerateBusinessID("MRFD"), req.OrderID, payment, buyer, req.RequestID, hex.EncodeToString(sum[:]), amount); err != nil {
		return hgCatalogDBError(err)
	}
	if err = hgOne(tx.ExecContext(ctx, SQLQueriesPackage.HGMallTradeRefundSQL, req.OrderID)); err != nil {
		return err
	}
	if err = hgOne(tx.ExecContext(ctx, SQLQueriesPackage.HGMallFulfillmentRefundSQL, req.OrderID)); err != nil {
		return err
	}
	return tx.Commit()
}
