package product

import (
	q "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"strconv"

	"github.com/go-sql-driver/mysql"
)

// HGStore 限定商家和商品目录的持久化职责，审核授权由现有 Ops RBAC 负责。
type HGStore interface {
	Merchant(context.Context, string) (HGMerchantApplication, error)
	Apply(context.Context, HGMerchantApplication) error
	ReviewMerchant(context.Context, string, string, int8) error
	PendingMerchants(context.Context, uint64) ([]HGMerchantApplication, error)
	Publish(context.Context, string, HGCatalogItem) error
	ReviewProduct(context.Context, string, string, int8) error
	List(context.Context, string, uint64) ([]HGCatalogItem, error)
	Detail(context.Context, string) (HGCatalogItem, error)
	Mine(context.Context, string, uint64) ([]HGCatalogItem, error)
	MineDetail(context.Context, string, string) (HGCatalogItem, error)
	Archive(context.Context, string, string) error
	StageEdit(context.Context, string, HGEditRequest) error
	PendingEdits(context.Context, uint64) ([]HGEditItem, error)
	ReviewEdit(context.Context, string, string, bool) error
	PendingProducts(context.Context, uint64) ([]HGCatalogItem, error)
	StageMedia(context.Context, string, string, []string) error
	PendingMedia(context.Context, uint64) ([]HGMediaItem, error)
	OwnPendingMedia(context.Context, string, string) ([]string, error)
	ReviewMedia(context.Context, string, string, bool) error
}

// HGRepository 使用共享 MySQL 池；商品和 SKU 在同一短事务创建，读取只走有界游标。
type HGRepository struct{ db *sql.DB }

func HGNewRepository(db *sql.DB) *HGRepository { return &HGRepository{db: db} }

func (r *HGRepository) Merchant(ctx context.Context, owner string) (HGMerchantApplication, error) {
	var m HGMerchantApplication
	err := r.db.QueryRowContext(ctx, q.HGMallMerchantByOwnerSQL, owner).Scan(&m.MerchantID, &m.ShopName, &m.Status)
	return m, err
}

func (r *HGRepository) Apply(ctx context.Context, m HGMerchantApplication) error {
	updated, err := r.db.ExecContext(ctx, q.HGMallMerchantReapplySQL, m.ShopName, m.UserID)
	if err != nil {
		return err
	}
	n, err := updated.RowsAffected()
	if err != nil {
		return err
	}
	if n == 1 {
		return nil
	}
	_, err = r.db.ExecContext(ctx, q.HGMallMerchantApplySQL, m.MerchantID, m.UserID, m.ShopName)
	return hgCatalogDBError(err)
}

func hgCatalogDBError(err error) error {
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		return ErrHGConflict
	}
	return err
}

func hgOne(result sql.Result, err error) error {
	if err != nil {
		return hgCatalogDBError(err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrHGConflict
	}
	return nil
}

func (r *HGRepository) ReviewMerchant(ctx context.Context, operator, merchantID string, status int8) error {
	return hgOne(r.db.ExecContext(ctx, q.HGMallMerchantApproveSQL, status, operator, merchantID, operator))
}

func (r *HGRepository) PendingMerchants(ctx context.Context, cursor uint64) ([]HGMerchantApplication, error) {
	if cursor == 0 {
		cursor = math.MaxUint64
	}
	rows, err := r.db.QueryContext(ctx, q.HGMallMerchantPendingSQL, cursor, 21)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := make([]HGMerchantApplication, 0, 21)
	for rows.Next() {
		var m HGMerchantApplication
		if err := rows.Scan(&m.ID, &m.MerchantID, &m.UserID, &m.ShopName); err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (r *HGRepository) Publish(ctx context.Context, owner string, item HGCatalogItem) (err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var merchantID string
	if err = tx.QueryRowContext(ctx, q.HGMallMerchantApprovedForUpdateSQL, owner).Scan(&merchantID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrHGForbidden
		}
		return err
	}
	if merchantID != item.MerchantID {
		return ErrHGForbidden
	}
	images, err := json.Marshal(item.ImageURLs)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, q.HGMallProductInsertSQL, item.ProductID, item.Type, item.Title, item.MerchantID, item.Category, item.Summary, item.Description, images); err != nil {
		return hgCatalogDBError(err)
	}
	seen := make(map[string]bool, len(item.ImageURLs))
	for _, imageURL := range item.ImageURLs {
		if seen[imageURL] {
			return ErrHGInvalid
		}
		seen[imageURL] = true
		if err = hgOne(tx.ExecContext(ctx, q.HGMallImageBindSQL, item.ProductID, imageURL, imageURL, owner)); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, q.HGMallSKUInsertSQL, item.SKUID, item.ProductID, item.SKUID, item.SKUTitle, item.Amount); err != nil {
		return hgCatalogDBError(err)
	}
	if item.Type == "PHYSICAL_PRODUCT" {
		if _, err = tx.ExecContext(ctx, q.HGMallInventoryInsertSQL, item.SKUID, item.Available); err != nil {
			return hgCatalogDBError(err)
		}
	}
	return tx.Commit()
}

func (r *HGRepository) ReviewProduct(ctx context.Context, operator, productID string, status int8) error {
	lifecycle := int8(0)
	if status == 1 {
		lifecycle = 1
	}
	return hgOne(r.db.ExecContext(ctx, q.HGMallProductReviewSQL, lifecycle, status, productID, operator))
}

func hgScanCatalog(row interface{ Scan(...any) error }, includeStatus bool) (HGCatalogItem, error) {
	var item HGCatalogItem
	var images []byte
	fields := []any{&item.ID, &item.ProductID, &item.Type, &item.Title, &item.Category, &item.Summary, &item.MerchantID, &item.ShopName, &item.SKUID, &item.SKUTitle, &item.Amount, &item.Available}
	if includeStatus {
		fields = append(fields, &item.Status, &item.LifecycleStatus)
	}
	fields = append(fields, &item.Description, &images)
	err := row.Scan(fields...)
	if err == nil && len(images) != 0 {
		err = json.Unmarshal(images, &item.ImageURLs)
	}
	if item.ImageURLs == nil {
		item.ImageURLs = []string{}
	}
	return item, err
}

func hgCatalogRows(rows *sql.Rows, includeStatus bool) ([]HGCatalogItem, error) {
	defer rows.Close()
	items := make([]HGCatalogItem, 0, 21)
	for rows.Next() {
		item, err := hgScanCatalog(rows, includeStatus)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *HGRepository) List(ctx context.Context, category string, cursor uint64) ([]HGCatalogItem, error) {
	var query string
	var args []any
	if category == "" {
		query, args = q.HGMallCatalogAllFirstSQL, []any{21}
		if cursor != 0 {
			query, args = q.HGMallCatalogAllNextSQL, []any{cursor, 21}
		}
	} else {
		query, args = q.HGMallCatalogFirstSQL, []any{category, 21}
		if cursor != 0 {
			query, args = q.HGMallCatalogNextSQL, []any{category, cursor, 21}
		}
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return hgCatalogRows(rows, false)
}

func (r *HGRepository) Detail(ctx context.Context, id string) (HGCatalogItem, error) {
	return hgScanCatalog(r.db.QueryRowContext(ctx, q.HGMallCatalogDetailSQL, id), false)
}

func (r *HGRepository) Mine(ctx context.Context, owner string, cursor uint64) ([]HGCatalogItem, error) {
	if cursor == 0 {
		cursor = math.MaxUint64
	}
	rows, err := r.db.QueryContext(ctx, q.HGMallMyProductsSQL, owner, cursor, 21)
	if err != nil {
		return nil, err
	}
	return hgCatalogRows(rows, true)
}

// MineDetail 使用商品业务唯一键与商家归属查单件，兼容已归档和待审商品。
func (r *HGRepository) MineDetail(ctx context.Context, owner, productID string) (HGCatalogItem, error) {
	return hgScanCatalog(r.db.QueryRowContext(ctx, q.HGMallMineByProductIDSQL, productID, owner), true)
}

// Archive 保留订单快照和实物，不再允许新订单；本人已归档商品再次操作返回冲突。
func (r *HGRepository) Archive(ctx context.Context, owner, productID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = hgOne(tx.ExecContext(ctx, q.HGMallMineArchiveSQL, productID, owner)); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, q.HGMallMineCancelMediaSQL, productID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, q.HGMallMineCancelEditSQL, productID); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *HGRepository) PendingProducts(ctx context.Context, cursor uint64) ([]HGCatalogItem, error) {
	if cursor == 0 {
		cursor = math.MaxUint64
	}
	rows, err := r.db.QueryContext(ctx, q.HGMallPendingProductsSQL, cursor, 21)
	if err != nil {
		return nil, err
	}
	return hgCatalogRows(rows, true)
}

func hgProductPage(items []HGCatalogItem) HGPage {
	page := HGPage{List: items, HasMore: len(items) > 20}
	if page.HasMore {
		page.List = items[:20]
		page.NextCursor = strconv.FormatUint(page.List[19].ID, 10)
	}
	return page
}
