package mallcny

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
	"time"

	"github.com/go-sql-driver/mysql"
)

// HGRepository 使用与商城及商家审核同一个 MySQL 连接池；CNY 交易不访问币账户。
type HGRepository struct {
	db *sql.DB /* 权威交易库，不是只读副本。 */
}

func HGNewRepository(db *sql.DB) *HGRepository { return &HGRepository{db: db} }

func hgProductScan(hgRow interface{ Scan(...any) error }, hgWithID bool) (HGProduct, uint64, error) {
	var hgItem HGProduct
	var hgID uint64
	var hgGroupPrice sql.NullInt64
	hgFields := []any{&hgItem.ProductID, &hgItem.MerchantID, &hgItem.Title, &hgItem.Description, &hgItem.ImageURL, &hgItem.SKUID, &hgItem.SKUTitle, &hgItem.PriceFen, &hgGroupPrice, &hgItem.GroupMembers, &hgItem.Available, &hgItem.ShopName}
	if hgWithID {
		hgFields = append([]any{&hgID}, hgFields...)
	}
	if err := hgRow.Scan(hgFields...); err != nil {
		return HGProduct{}, 0, err
	}
	if hgGroupPrice.Valid {
		hgItem.GroupPriceFen = &hgGroupPrice.Int64
	}
	return hgItem, hgID, nil
}

// HGList 每次至多读 21 条并使用 (status,id) 索引；下一页只携带最后一条已展示 ID。
func (r *HGRepository) HGList(ctx context.Context, cursor uint64) ([]HGProduct, uint64, error) {
	hgRows, err := r.db.QueryContext(ctx, q.HGCNYListSQL, cursor, 21)
	if err != nil {
		return nil, 0, err
	}
	defer hgRows.Close()
	hgItems := make([]HGProduct, 0, 20)
	var hgLast uint64
	hgHasMore := false
	for hgRows.Next() {
		hgItem, hgID, hgErr := hgProductScan(hgRows, true)
		if hgErr != nil {
			return nil, 0, hgErr
		}
		if len(hgItems) == 20 {
			hgHasMore = true
			break
		}
		hgItems = append(hgItems, hgItem)
		hgLast = hgID
	}
	if err = hgRows.Err(); err != nil {
		return nil, 0, err
	}
	if !hgHasMore {
		return hgItems, 0, nil
	}
	return hgItems, hgLast, nil
}

func (r *HGRepository) HGDetail(ctx context.Context, id string) (HGProduct, error) {
	hgItem, _, err := hgProductScan(r.db.QueryRowContext(ctx, q.HGCNYDetailSQL, id), false)
	if errors.Is(err, sql.ErrNoRows) {
		return HGProduct{}, ErrHGNotFound
	}
	return hgItem, err
}

func hgOne(hgResult sql.Result, err error) error {
	if err != nil {
		return err
	}
	hgCount, err := hgResult.RowsAffected()
	if err != nil {
		return err
	}
	if hgCount != 1 {
		return ErrHGConflict
	}
	return nil
}

func (r *HGRepository) HGPublish(ctx context.Context, user string, req HGCreateProduct) (HGProduct, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return HGProduct{}, err
	}
	defer tx.Rollback()
	var hgMerchant string
	if err = tx.QueryRowContext(ctx, q.HGCNYMerchantSQL, user).Scan(&hgMerchant); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return HGProduct{}, ErrHGNotFound
		}
		return HGProduct{}, err
	}
	hgProductID, hgSKUID := util.GenerateBusinessID("CNYPRD"), util.GenerateBusinessID("CNYSKU")
	if _, err = tx.ExecContext(ctx, q.HGCNYProductInsertSQL, hgProductID, hgMerchant, req.Title, req.Description, req.ImageURL, hgSKUID, req.SKUTitle, req.PriceFen, req.GroupPriceFen, req.GroupMembers); err != nil {
		return HGProduct{}, err
	}
	if _, err = tx.ExecContext(ctx, q.HGCNYInventoryInsertSQL, hgSKUID, req.Stock); err != nil {
		return HGProduct{}, err
	}
	if err = tx.Commit(); err != nil {
		return HGProduct{}, err
	}
	return r.HGDetail(ctx, hgProductID)
}

func (r *HGRepository) HGCreateCoupon(ctx context.Context, user string, req HGCreateCoupon) (HGCoupon, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return HGCoupon{}, err
	}
	defer tx.Rollback()
	var hgMerchant string
	if err = tx.QueryRowContext(ctx, q.HGCNYMerchantSQL, user).Scan(&hgMerchant); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return HGCoupon{}, ErrHGNotFound
		}
		return HGCoupon{}, err
	}
	var hgExists int
	if err = tx.QueryRowContext(ctx, q.HGCNYMerchantProductSQL, req.ProductID, hgMerchant).Scan(&hgExists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return HGCoupon{}, ErrHGNotFound
		}
		return HGCoupon{}, err
	}
	hgCoupon := HGCoupon{CouponID: util.GenerateBusinessID("CNYCOU"), ProductID: req.ProductID, ThresholdFen: req.ThresholdFen, DiscountFen: req.DiscountFen, Remaining: req.Quantity, ExpiresAt: req.ExpiresAt.UTC()}
	_, err = tx.ExecContext(ctx, q.HGCNYCouponInsertSQL, hgCoupon.CouponID, hgCoupon.ProductID, hgCoupon.ThresholdFen, hgCoupon.DiscountFen, hgCoupon.Remaining, hgCoupon.ExpiresAt)
	if err != nil {
		return HGCoupon{}, err
	}
	return hgCoupon, tx.Commit()
}

func (r *HGRepository) HGCoupons(ctx context.Context, productID string) ([]HGCoupon, error) {
	hgRows, err := r.db.QueryContext(ctx, q.HGCNYCouponListSQL, productID)
	if err != nil {
		return nil, err
	}
	defer hgRows.Close()
	hgList := make([]HGCoupon, 0)
	for hgRows.Next() {
		var hgCoupon HGCoupon
		err = hgRows.Scan(&hgCoupon.CouponID, &hgCoupon.ProductID, &hgCoupon.ThresholdFen, &hgCoupon.DiscountFen, &hgCoupon.Remaining, &hgCoupon.ExpiresAt)
		if err != nil {
			return nil, err
		}
		hgList = append(hgList, hgCoupon)
	}
	return hgList, hgRows.Err()
}

func (r *HGRepository) HGClaimCoupon(ctx context.Context, user, productID, couponID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = hgOne(tx.ExecContext(ctx, q.HGCNYCouponClaimSQL, couponID, productID)); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, q.HGCNYUserCouponInsertSQL, user, couponID); err != nil {
		var hgDuplicate *mysql.MySQLError
		if errors.As(err, &hgDuplicate) && hgDuplicate.Number == 1062 {
			return ErrHGConflict
		}
		return err
	}
	return tx.Commit()
}

func hgGroupScan(hgRow interface{ Scan(...any) error }) (HGGroup, error) {
	var hgGroup HGGroup
	err := hgRow.Scan(&hgGroup.GroupID, &hgGroup.ProductID, &hgGroup.TargetMembers, &hgGroup.PaidMembers, &hgGroup.ReservedMembers, &hgGroup.Status, &hgGroup.ExpiresAt)
	return hgGroup, err
}

func (r *HGRepository) HGGroups(ctx context.Context, productID string) ([]HGGroup, error) {
	hgRows, err := r.db.QueryContext(ctx, q.HGCNYGroupListSQL, productID)
	if err != nil {
		return nil, err
	}
	defer hgRows.Close()
	hgGroups := make([]HGGroup, 0)
	for hgRows.Next() {
		hgGroup, hgErr := hgGroupScan(hgRows)
		if hgErr != nil {
			return nil, hgErr
		}
		hgGroups = append(hgGroups, hgGroup)
	}
	return hgGroups, hgRows.Err()
}

func (r *HGRepository) HGGroup(ctx context.Context, id string) (HGGroup, error) {
	hgGroup, err := hgGroupScan(r.db.QueryRowContext(ctx, q.HGCNYGroupSQL, id))
	if errors.Is(err, sql.ErrNoRows) {
		return HGGroup{}, ErrHGNotFound
	}
	return hgGroup, err
}

// hgQuote 在读快照上计算预览；创建订单在事务内重新取价格、券、库存并比较期望金额。
func (r *HGRepository) hgQuote(ctx context.Context, hgQuery interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, user string, req HGOrderRequest, product HGProduct) (HGQuote, error) {
	if req.ProductID != product.ProductID || req.SKUID != product.SKUID || req.Quantity < 1 || req.Quantity > 10 {
		return HGQuote{}, ErrHGInvalid
	}
	hgUnit := product.PriceFen
	if req.PurchaseType != "single" {
		if product.GroupPriceFen == nil || product.GroupMembers < 2 || req.CouponID != "" {
			return HGQuote{}, ErrHGInvalid
		}
		hgUnit = *product.GroupPriceFen
	}
	if hgUnit < 1 || hgUnit > math.MaxInt64/int64(req.Quantity) || hgUnit*int64(req.Quantity) > 10000000 {
		return HGQuote{}, ErrHGInvalid
	}
	hgQuote := HGQuote{UnitFen: hgUnit, TotalFen: hgUnit * int64(req.Quantity), Available: product.Available >= uint32(req.Quantity)}
	if req.CouponID != "" {
		var hgThreshold, hgDiscount int64
		if err := hgQuery.QueryRowContext(ctx, q.HGCNYOwnedCouponSQL, user, req.CouponID, req.ProductID).Scan(&hgThreshold, &hgDiscount); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return HGQuote{}, ErrHGConflict
			}
			return HGQuote{}, err
		}
		if hgQuote.TotalFen < hgThreshold || hgDiscount >= hgQuote.TotalFen {
			return HGQuote{}, ErrHGConflict
		}
		hgQuote.DiscountFen = hgDiscount
	}
	hgQuote.PayableFen = hgQuote.TotalFen - hgQuote.DiscountFen
	return hgQuote, nil
}

func (r *HGRepository) HGPreview(ctx context.Context, user string, req HGOrderRequest) (HGQuote, error) {
	hgItem, err := r.HGDetail(ctx, req.ProductID)
	if err != nil {
		return HGQuote{}, err
	}
	if req.PurchaseType == "group_join" {
		hgGroup, hgErr := r.HGGroup(ctx, req.GroupID)
		if hgErr != nil {
			return HGQuote{}, hgErr
		}
		if hgGroup.ProductID != req.ProductID || hgGroup.Status != 1 || !hgGroup.ExpiresAt.After(time.Now().UTC()) || hgGroup.PaidMembers+hgGroup.ReservedMembers >= hgGroup.TargetMembers {
			return HGQuote{}, ErrHGConflict
		}
	}
	return r.hgQuote(ctx, r.db, user, req, hgItem)
}

func hgHash(req HGOrderRequest) string {
	hgData, _ := json.Marshal(req)
	hgDigest := sha256.Sum256(hgData)
	return hex.EncodeToString(hgDigest[:])
}

func hgOrderScan(hgRow interface{ Scan(...any) error }, hgHashField, hgSKUField *string) (HGOrder, error) {
	var hgOrder HGOrder
	hgFields := []any{&hgOrder.OrderID, &hgOrder.ProductID, &hgOrder.Title, &hgOrder.Quantity, &hgOrder.PurchaseType, &hgOrder.GroupID, &hgOrder.CouponID, &hgOrder.TotalFen, &hgOrder.DiscountFen, &hgOrder.PayableFen, &hgOrder.Status, &hgOrder.ExpiresAt}
	if hgHashField != nil {
		hgFields = append(hgFields, hgHashField)
	}
	if hgSKUField != nil {
		hgFields = append(hgFields, hgSKUField)
	}
	return hgOrder, hgRow.Scan(hgFields...)
}

func (r *HGRepository) HGOrder(ctx context.Context, user, id string) (HGOrder, error) {
	hgOrder, err := hgOrderScan(r.db.QueryRowContext(ctx, q.HGCNYOrderOwnerSQL, id, user), nil, nil)
	if errors.Is(err, sql.ErrNoRows) {
		return HGOrder{}, ErrHGNotFound
	}
	return hgOrder, err
}

// HGBuy 锁定团席/券后再条件更新库存，并在同一事务插入订单；唯一键处理并发同键重放。
func (r *HGRepository) HGBuy(ctx context.Context, user string, req HGOrderRequest) (HGOrder, error) {
	hgFingerprint := hgHash(req)
	hgReplay := func() (HGOrder, error) {
		var hgOldHash string
		hgOld, hgErr := hgOrderScan(r.db.QueryRowContext(ctx, q.HGCNYOrderReplaySQL, user, req.RequestID), &hgOldHash, nil)
		if hgErr == nil && hgOldHash != hgFingerprint {
			return HGOrder{}, ErrHGConflict
		}
		return hgOld, hgErr
	}
	if hgOld, hgErr := hgReplay(); hgErr == nil {
		return hgOld, nil
	} else if !errors.Is(hgErr, sql.ErrNoRows) {
		return HGOrder{}, hgErr
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return HGOrder{}, err
	}
	defer tx.Rollback()
	// 点查商品及在售商家；币种永远由独立表 CNY 契约确定。
	hgProduct, _, err := hgProductScan(tx.QueryRowContext(ctx, q.HGCNYDetailSQL, req.ProductID), false)
	if err != nil {
		return HGOrder{}, ErrHGNotFound
	}
	var hgSeller int
	if err = tx.QueryRowContext(ctx, q.HGCNYMerchantOwnerSQL, hgProduct.MerchantID, user).Scan(&hgSeller); err == nil {
		return HGOrder{}, ErrHGConflict
	} else if !errors.Is(err, sql.ErrNoRows) {
		return HGOrder{}, err
	}
	hgOrderID := util.GenerateBusinessID("CNYORD")
	var hgGroupID string
	hgExpiry := time.Now().UTC().Add(15 * time.Minute)
	if req.PurchaseType == "group_join" {
		var hgGroup HGGroup
		hgGroup, err = hgGroupScan(tx.QueryRowContext(ctx, q.HGCNYGroupLockSQL, req.GroupID))
		if err != nil {
			return HGOrder{}, ErrHGConflict
		}
		if hgGroup.ProductID != req.ProductID || hgGroup.Status != 1 || !hgGroup.ExpiresAt.After(time.Now().UTC()) || hgGroup.PaidMembers+hgGroup.ReservedMembers >= hgGroup.TargetMembers {
			return HGOrder{}, ErrHGConflict
		}
		if err = hgOne(tx.ExecContext(ctx, q.HGCNYGroupReserveSQL, req.GroupID, req.ProductID)); err != nil {
			return HGOrder{}, err
		}
		hgGroupID = req.GroupID
		if hgGroup.ExpiresAt.Before(hgExpiry) {
			hgExpiry = hgGroup.ExpiresAt
		}
	} else if req.PurchaseType == "group_create" {
		if hgProduct.GroupMembers < 2 {
			return HGOrder{}, ErrHGConflict
		}
		hgGroupID = util.GenerateBusinessID("CNYGRP")
		// 团长模拟支付前不开放招募；到期时间在团长支付后可由正式渠道重新设计。
		_, err = tx.ExecContext(ctx, q.HGCNYGroupInsertSQL, hgGroupID, req.ProductID, user, hgProduct.GroupMembers, hgExpiry.Add(24*time.Hour))
		if err != nil {
			return HGOrder{}, err
		}
	}
	hgQuote, err := r.hgQuote(ctx, tx, user, req, hgProduct)
	if err != nil {
		return HGOrder{}, err
	}
	if req.ExpectedPayableFen != hgQuote.PayableFen {
		return HGOrder{}, ErrHGQuoteChanged
	}
	if err = hgOne(tx.ExecContext(ctx, q.HGCNYReserveStockSQL, req.Quantity, req.Quantity, req.SKUID, req.Quantity)); err != nil {
		return HGOrder{}, err
	}
	if req.CouponID != "" {
		if err = hgOne(tx.ExecContext(ctx, q.HGCNYLockCouponSQL, hgOrderID, user, req.CouponID)); err != nil {
			return HGOrder{}, err
		}
	}
	var hgNullableGroup, hgNullableCoupon any
	if hgGroupID != "" {
		hgNullableGroup = hgGroupID
	}
	if req.CouponID != "" {
		hgNullableCoupon = req.CouponID
	}
	_, err = tx.ExecContext(ctx, q.HGCNYOrderInsertSQL, hgOrderID, user, req.RequestID, hgFingerprint, req.ProductID, req.SKUID, hgProduct.Title, req.Quantity, req.PurchaseType, hgNullableGroup, hgNullableCoupon, hgQuote.TotalFen, hgQuote.DiscountFen, hgQuote.PayableFen, req.Address, hgExpiry)
	if err != nil {
		var hgDuplicate *mysql.MySQLError
		if errors.As(err, &hgDuplicate) && hgDuplicate.Number == 1062 {
			_ = tx.Rollback()
			if hgOld, hgErr := hgReplay(); hgErr == nil {
				return hgOld, nil
			}
			return HGOrder{}, ErrHGConflict
		}
		return HGOrder{}, err
	}
	if err = tx.Commit(); err != nil {
		return HGOrder{}, err
	}
	return r.HGOrder(ctx, user, hgOrderID)
}

// HGOrders 按本人和内部 ID 倒序取最多 20 单；地址快照不会在列表中返回。
func (r *HGRepository) HGOrders(ctx context.Context, user string, cursor uint64) ([]HGOrder, uint64, error) {
	// 订单列表查询必须包含 id 才能生成下一页游标；在限定 21 单内单独取游标避免向外暴露。
	hgRows, err := r.db.QueryContext(ctx, q.HGCNYOrdersSQL, user, cursor)
	if err != nil {
		return nil, 0, err
	}
	defer hgRows.Close()
	hgItems := make([]HGOrder, 0, 20)
	var hgLast uint64
	hgHasMore := false
	for hgRows.Next() {
		var hgID uint64
		var hgOrder HGOrder
		hgErr := hgRows.Scan(&hgID, &hgOrder.OrderID, &hgOrder.ProductID, &hgOrder.Title, &hgOrder.Quantity, &hgOrder.PurchaseType, &hgOrder.GroupID, &hgOrder.CouponID, &hgOrder.TotalFen, &hgOrder.DiscountFen, &hgOrder.PayableFen, &hgOrder.Status, &hgOrder.ExpiresAt)
		if hgErr != nil {
			return nil, 0, hgErr
		}
		if len(hgItems) == 20 {
			hgHasMore = true
			break
		}
		hgItems = append(hgItems, hgOrder)
		hgLast = hgID
	}
	if err = hgRows.Err(); err != nil {
		return nil, 0, err
	}
	if !hgHasMore {
		return hgItems, 0, nil
	}
	return hgItems, hgLast, nil
}

func (r *HGRepository) HGDebugPay(ctx context.Context, user, id string) (HGOrder, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return HGOrder{}, err
	}
	defer tx.Rollback()
	var hgSKU string
	hgOrder, err := hgOrderScan(tx.QueryRowContext(ctx, q.HGCNYOrderLockSQL, id, user), nil, &hgSKU)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return HGOrder{}, ErrHGNotFound
		}
		return HGOrder{}, err
	}
	if hgOrder.Status == 1 || hgOrder.Status == 2 {
		return hgOrder, nil
	}
	if hgOrder.Status != 0 || !hgOrder.ExpiresAt.After(time.Now().UTC()) {
		return HGOrder{}, ErrHGConflict
	}
	if hgOrder.GroupID != "" {
		if err = hgOne(tx.ExecContext(ctx, q.HGCNYGroupPaidSQL, hgOrder.GroupID)); err != nil {
			return HGOrder{}, err
		}
	}
	if err = hgOne(tx.ExecContext(ctx, q.HGCNYCommitStockSQL, hgOrder.Quantity, hgOrder.Quantity, hgSKU, hgOrder.Quantity)); err != nil {
		return HGOrder{}, err
	}
	if hgOrder.CouponID != "" {
		if err = hgOne(tx.ExecContext(ctx, q.HGCNYCouponSpendSQL, user, hgOrder.CouponID, id)); err != nil {
			return HGOrder{}, err
		}
	}
	hgStatus := uint8(1)
	if hgOrder.GroupID != "" {
		hgGroup, hgErr := hgGroupScan(tx.QueryRowContext(ctx, q.HGCNYGroupLockSQL, hgOrder.GroupID))
		if hgErr != nil {
			return HGOrder{}, hgErr
		}
		if hgGroup.Status != 2 {
			hgStatus = 2
		}
	}
	if err = hgOne(tx.ExecContext(ctx, q.HGCNYOrderPaidSQL, hgStatus, id, user)); err != nil {
		return HGOrder{}, err
	}
	if _, err = tx.ExecContext(ctx, q.HGCNYPaymentInsertSQL, id, util.GenerateBusinessID("CNYPAY"), user, hgOrder.PayableFen); err != nil {
		return HGOrder{}, err
	}
	if hgOrder.GroupID != "" && hgStatus == 1 {
		if _, err = tx.ExecContext(ctx, q.HGCNYGroupOrdersReadySQL, hgOrder.GroupID); err != nil {
			return HGOrder{}, err
		}
	}
	if err = tx.Commit(); err != nil {
		return HGOrder{}, err
	}
	return r.HGOrder(ctx, user, id)
}

// HGCancel 仅取消本人未支付订单；库存、券和团名额在同一短事务释放。
func (r *HGRepository) HGCancel(ctx context.Context, user, id string) (HGOrder, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return HGOrder{}, err
	}
	defer tx.Rollback()
	var hgSKU string
	hgOrder, err := hgOrderScan(tx.QueryRowContext(ctx, q.HGCNYOrderLockSQL, id, user), nil, &hgSKU)
	if err != nil {
		return HGOrder{}, ErrHGNotFound
	}
	if hgOrder.Status == 3 {
		return hgOrder, nil
	}
	if hgOrder.Status != 0 {
		return HGOrder{}, ErrHGConflict
	}
	if hgOrder.GroupID != "" {
		if err = hgOne(tx.ExecContext(ctx, q.HGCNYGroupReleaseSQL, hgOrder.GroupID)); err != nil {
			return HGOrder{}, err
		}
	}
	if err = hgOne(tx.ExecContext(ctx, q.HGCNYReleaseStockSQL, hgOrder.Quantity, hgOrder.Quantity, hgSKU, hgOrder.Quantity)); err != nil {
		return HGOrder{}, err
	}
	if hgOrder.CouponID != "" {
		if err = hgOne(tx.ExecContext(ctx, q.HGCNYCouponReleaseSQL, user, hgOrder.CouponID, id)); err != nil {
			return HGOrder{}, err
		}
	}
	if err = hgOne(tx.ExecContext(ctx, q.HGCNYOrderCancelSQL, id, user)); err != nil {
		return HGOrder{}, err
	}
	if err = tx.Commit(); err != nil {
		return HGOrder{}, err
	}
	return r.HGOrder(ctx, user, id)
}

// HGDebugRefund 限制为已模拟支付且未发货或拼团失败的整单原路模拟退款，唯一索引防重复。
func (r *HGRepository) HGDebugRefund(ctx context.Context, user, id string) (HGOrder, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return HGOrder{}, err
	}
	defer tx.Rollback()
	var hgSKU string
	hgOrder, err := hgOrderScan(tx.QueryRowContext(ctx, q.HGCNYOrderLockSQL, id, user), nil, &hgSKU)
	if err != nil {
		return HGOrder{}, ErrHGNotFound
	}
	if hgOrder.Status == 4 {
		return hgOrder, nil
	}
	if hgOrder.Status != 1 && hgOrder.Status != 2 {
		return HGOrder{}, ErrHGConflict
	}
	var hgPaidAmount int64
	if err = tx.QueryRowContext(ctx, q.HGCNYPaymentAmountSQL, id, user).Scan(&hgPaidAmount); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return HGOrder{}, ErrHGConflict
		}
		return HGOrder{}, err
	}
	if hgPaidAmount != hgOrder.PayableFen {
		return HGOrder{}, ErrHGConflict
	}
	if hgOrder.GroupID != "" {
		hgGroup, hgErr := hgGroupScan(tx.QueryRowContext(ctx, q.HGCNYGroupLockSQL, hgOrder.GroupID))
		if hgErr != nil {
			return HGOrder{}, hgErr
		}
		if hgGroup.Status != 3 && hgGroup.Status != 4 && hgGroup.Status != 5 {
			return HGOrder{}, ErrHGConflict
		}
	}
	if err = hgOne(tx.ExecContext(ctx, q.HGCNYRestockSQL, hgOrder.Quantity, hgOrder.Quantity, hgSKU, hgOrder.Quantity)); err != nil {
		return HGOrder{}, err
	}
	if err = hgOne(tx.ExecContext(ctx, q.HGCNYOrderRefundSQL, id, user)); err != nil {
		return HGOrder{}, err
	}
	if _, err = tx.ExecContext(ctx, q.HGCNYRefundInsertSQL, id, util.GenerateBusinessID("CNYREF"), user, hgOrder.PayableFen); err != nil {
		return HGOrder{}, err
	}
	if err = tx.Commit(); err != nil {
		return HGOrder{}, err
	}
	return r.HGOrder(ctx, user, id)
}

// HGExpiryBatch 有界推进订单取消、拼团到期及退款；跨 Pod 并发以订单行锁和唯一索引幂等。
func (r *HGRepository) HGExpiryBatch(ctx context.Context) error {
	hgExpired, err := r.db.QueryContext(ctx, q.HGCNYExpiredOrderBatchSQL)
	if err != nil {
		return err
	}
	type hgOwnerOrder struct {
		id, user string
		status   uint8
	}
	hgOrders := make([]hgOwnerOrder, 0, 20)
	for hgExpired.Next() {
		var hgItem hgOwnerOrder
		if err = hgExpired.Scan(&hgItem.id, &hgItem.user); err != nil {
			_ = hgExpired.Close()
			return err
		}
		hgOrders = append(hgOrders, hgItem)
	}
	err = hgExpired.Err()
	_ = hgExpired.Close()
	if err != nil {
		return err
	}
	for _, hgItem := range hgOrders {
		if _, err = r.HGCancel(ctx, hgItem.user, hgItem.id); err != nil && !errors.Is(err, ErrHGConflict) {
			return err
		}
	}
	hgExpiring, err := r.db.QueryContext(ctx, q.HGCNYExpiredGroupBatchSQL)
	if err != nil {
		return err
	}
	hgGroups := make([]string, 0, 10)
	for hgExpiring.Next() {
		var hgID string
		if err = hgExpiring.Scan(&hgID); err != nil {
			_ = hgExpiring.Close()
			return err
		}
		hgGroups = append(hgGroups, hgID)
	}
	err = hgExpiring.Err()
	_ = hgExpiring.Close()
	if err != nil {
		return err
	}
	for _, hgID := range hgGroups {
		if _, err = r.db.ExecContext(ctx, q.HGCNYGroupFailSQL, hgID); err != nil {
			return err
		}
	}
	hgFailed, err := r.db.QueryContext(ctx, q.HGCNYFailedGroupBatchSQL)
	if err != nil {
		return err
	}
	hgGroups = hgGroups[:0]
	for hgFailed.Next() {
		var hgID string
		if err = hgFailed.Scan(&hgID); err != nil {
			_ = hgFailed.Close()
			return err
		}
		hgGroups = append(hgGroups, hgID)
	}
	err = hgFailed.Err()
	_ = hgFailed.Close()
	if err != nil {
		return err
	}
	for _, hgID := range hgGroups {
		if err = r.hgSettleFailedGroup(ctx, hgID); err != nil {
			return err
		}
	}
	return nil
}

func (r *HGRepository) hgSettleFailedGroup(ctx context.Context, groupID string) error {
	hgRows, err := r.db.QueryContext(ctx, q.HGCNYGroupOpenOrdersSQL, groupID)
	if err != nil {
		return err
	}
	hgOrders := make([]struct {
		id, user string
		status   uint8
	}, 0, 10)
	for hgRows.Next() {
		var hgItem struct {
			id, user string
			status   uint8
		}
		if err = hgRows.Scan(&hgItem.id, &hgItem.user, &hgItem.status); err != nil {
			_ = hgRows.Close()
			return err
		}
		hgOrders = append(hgOrders, hgItem)
	}
	err = hgRows.Err()
	_ = hgRows.Close()
	if err != nil {
		return err
	}
	if len(hgOrders) > 10 {
		return ErrHGConflict
	}
	for _, hgItem := range hgOrders {
		if hgItem.status == 0 {
			_, err = r.HGCancel(ctx, hgItem.user, hgItem.id)
		} else {
			_, err = r.HGDebugRefund(ctx, hgItem.user, hgItem.id)
		}
		if err != nil && !errors.Is(err, ErrHGConflict) {
			return err
		}
	}
	_, err = r.db.ExecContext(ctx, q.HGCNYGroupSettledSQL, groupID, groupID)
	return err
}
