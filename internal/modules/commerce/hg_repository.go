package commerce

import (
	q "MLC_GO/internal/pkg/mysql/queries"
	util "MLC_GO/internal/pkg/utils"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"math"

	"github.com/go-sql-driver/mysql"
)

// HGRepository 使用共享 MySQL 连接池；写操作在同一短事务内验证身份、版本和状态。
type HGRepository struct{ db *sql.DB }

// HGNewRepository 创建生产 CNY 商品仓储；不访问 debug 沙箱表或旧币商品。
func HGNewRepository(db *sql.DB) *HGRepository { return &HGRepository{db: db} }

func hgDBError(err error) error {
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		return ErrHGConflict
	}
	return err
}

func (r *HGRepository) hgMerchant(ctx context.Context, tx *sql.Tx, user string) (string, error) {
	var merchant string
	err := tx.QueryRowContext(ctx, q.HGCommerceMerchantLockSQL, user).Scan(&merchant)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrHGForbidden
	}
	return merchant, err
}

// HGCategories 仅允许已核准商家读取动态普通叶子类目，受监管类目不参与当前发布。
func (r *HGRepository) HGCategories(ctx context.Context, user string) ([]HGCategory, error) {
	var merchant string
	if err := r.db.QueryRowContext(ctx, q.HGCommerceMerchantSQL, user).Scan(&merchant); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrHGMerchantRequired
		}
		return nil, err
	}
	return r.HGAdminCategories(ctx)
}

// HGAdminCategories 读取平台普通叶子类目；调用方必须先通过审核权限校验。
func (r *HGRepository) HGAdminCategories(ctx context.Context) ([]HGCategory, error) {
	rows, err := r.db.QueryContext(ctx, q.HGCommerceCategorySQL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]HGCategory, 0, 20)
	for rows.Next() {
		var parent sql.NullString
		var item HGCategory
		if err = rows.Scan(&item.ID, &item.Name, &parent, &item.Depth, &item.ImageURL); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// HGCreateCategory 仅支持最多三级的平台配置；不预置发布白名单或凭空授予资质。
func (r *HGRepository) HGCreateCategory(ctx context.Context, user string, request HGCategoryRequest) (HGCategory, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return HGCategory{}, err
	}
	defer tx.Rollback()
	depth := uint8(1)
	var parent any
	if request.ParentID != "" {
		id, parseErr := hgCategoryID(request.ParentID)
		if parseErr != nil {
			return HGCategory{}, parseErr
		}
		var parentDepth uint8
		var regulated bool
		if err = tx.QueryRowContext(ctx, q.HGCommerceCategoryParentSQL, id).Scan(&parentDepth, &regulated); err != nil {
			return HGCategory{}, err
		}
		if regulated && !request.Regulated {
			return HGCategory{}, ErrHGCategoryParentInvalid
		}
		if parentDepth >= 3 {
			return HGCategory{}, ErrHGCategoryParentInvalid
		}
		depth = parentDepth + 1
		parent = id
	}
	_, err = tx.ExecContext(ctx, q.HGCommerceCategoryInsertSQL, request.CategoryID, parent, depth, request.Name, request.Regulated, request.ImageURL)
	if err != nil {
		return HGCategory{}, hgDBError(err)
	}
	if request.ImageURL != "" {
		if err = hgExactlyOne(tx.ExecContext(ctx, q.HGCommerceCategoryMediaBindSQL, request.CategoryID, request.ImageURL, request.ImageURL, user)); err != nil {
			if errors.Is(err, ErrHGConflict) {
				return HGCategory{}, ErrHGCategoryImageInvalid
			}
			return HGCategory{}, err
		}
	}
	if err = tx.Commit(); err != nil {
		return HGCategory{}, err
	}
	return HGCategory{ID: request.CategoryID, Name: request.Name, Depth: depth, ImageURL: request.ImageURL}, nil
}

func hgValidateCategory(ctx context.Context, tx *sql.Tx, category string) error {
	var exists string
	err := tx.QueryRowContext(ctx, q.HGCommerceCategoryLeafSQL, category).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrHGInvalid
	}
	return err
}

func hgProductState(row interface{ Scan(...any) error }) (HGProduct, error) {
	var p HGProduct
	err := row.Scan(&p.ID, &p.ProductID, &p.MerchantID, &p.CategoryID, &p.AuditStatus, &p.SaleStatus, &p.Version, &p.DraftVersion, &p.PublishedVersion)
	p.Currency = "CNY"
	p.ProductType = "PHYSICAL"
	return p, err
}

func hgOwnerState(row interface{ Scan(...any) error }, id string) (HGProduct, error) {
	var p HGProduct
	err := row.Scan(&p.MerchantID, &p.CategoryID, &p.AuditStatus, &p.SaleStatus, &p.Version, &p.DraftVersion, &p.PublishedVersion)
	p.ProductID = id
	p.Currency = "CNY"
	p.ProductType = "PHYSICAL"
	if errors.Is(err, sql.ErrNoRows) {
		return HGProduct{}, ErrHGForbidden
	}
	return p, err
}

func hgWriteVersion(ctx context.Context, tx *sql.Tx, user, id string, version uint64, d HGDraftInput) error {
	images, _ := json.Marshal(d.ImageURLs)
	if _, err := tx.ExecContext(ctx, q.HGCommerceCreateVersionSQL, id, version, d.Title, d.Subtitle, d.Description, images); err != nil {
		return hgDBError(err)
	}
	for _, url := range d.ImageURLs {
		// 已绑定本商品的图片可以继续被本商品的新审核版本引用；不得冒用另一商品或商家的 URL。
		var owned string
		err := tx.QueryRowContext(ctx, q.HGCommerceMediaReuseSQL, url, url, user, id).Scan(&owned)
		if errors.Is(err, sql.ErrNoRows) {
			err = hgExactlyOne(tx.ExecContext(ctx, q.HGCommerceBindMediaSQL, id, url, url, user))
		}
		if err != nil {
			return err
		}
	}
	for _, sku := range d.SKUs {
		data, _ := json.Marshal(sku.Specs)
		hash := sha256.Sum256(data)
		if _, err := tx.ExecContext(ctx, q.HGCommerceCreateSKUSQL, util.GenerateBusinessID("CSKU"), id, version, sku.SKUCode, data, hash[:], sku.PriceCent, sku.InitialStock); err != nil {
			return hgDBError(err)
		}
	}
	return nil
}

// HGCreateDraft 串行化同店创建，幂等键、资产绑定、商品版本和 SKU 同一事务提交。
func (r *HGRepository) HGCreateDraft(ctx context.Context, user, requestID string, d HGDraftInput) (HGProduct, error) {
	data, _ := json.Marshal(d)
	hash := sha256.Sum256(data)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return HGProduct{}, err
	}
	defer tx.Rollback()
	merchant, err := r.hgMerchant(ctx, tx, user)
	if err != nil {
		return HGProduct{}, err
	}
	var oldHash []byte
	var existing string
	err = tx.QueryRowContext(ctx, q.HGCommerceCreateReplaySQL, merchant, requestID).Scan(&oldHash, &existing)
	if err == nil {
		if string(oldHash) != string(hash[:]) {
			return HGProduct{}, ErrHGConflict
		}
		_ = tx.Rollback()
		return r.HGDetail(ctx, user, existing, "mine")
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return HGProduct{}, err
	}
	if err = hgValidateCategory(ctx, tx, d.CategoryID); err != nil {
		return HGProduct{}, err
	}
	id := util.GenerateBusinessID("CPRD")
	if _, err = tx.ExecContext(ctx, q.HGCommerceCreateProductSQL, id, merchant, d.CategoryID); err != nil {
		return HGProduct{}, hgDBError(err)
	}
	if err = hgWriteVersion(ctx, tx, user, id, 1, d); err != nil {
		return HGProduct{}, err
	}
	if _, err = tx.ExecContext(ctx, q.HGCommerceCreateKeySQL, merchant, requestID, hash[:], id); err != nil {
		return HGProduct{}, hgDBError(err)
	}
	if err = tx.Commit(); err != nil {
		return HGProduct{}, err
	}
	return r.HGDetail(ctx, user, id, "mine")
}

// HGEditDraft 写新版本并保留旧的 publishedVersion，禁止归档或待审版本被覆盖。
func (r *HGRepository) HGEditDraft(ctx context.Context, user, id string, d HGDraftInput) (HGProduct, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return HGProduct{}, err
	}
	defer tx.Rollback()
	p, err := hgOwnerState(tx.QueryRowContext(ctx, q.HGCommerceOwnerLockSQL, id, user), id)
	if err != nil {
		return HGProduct{}, err
	}
	if p.Version != d.ExpectedVersion || p.AuditStatus == "PENDING" || p.SaleStatus == "DELETED" {
		return HGProduct{}, ErrHGConflict
	}
	if p.PublishedVersion != 0 && d.CategoryID != p.CategoryID {
		return HGProduct{}, ErrHGInvalid
	} // 在售类目不允许未审核切换。
	if err = hgValidateCategory(ctx, tx, d.CategoryID); err != nil {
		return HGProduct{}, err
	}
	if p.DraftVersion == math.MaxUint64 {
		return HGProduct{}, ErrHGConflict
	}
	version := p.DraftVersion + 1
	if err = hgExactlyOne(tx.ExecContext(ctx, q.HGCommerceProductCategorySQL, d.CategoryID, version, id, p.Version)); err != nil {
		return HGProduct{}, err
	}
	if err = hgWriteVersion(ctx, tx, user, id, version, d); err != nil {
		return HGProduct{}, err
	}
	if err = tx.Commit(); err != nil {
		return HGProduct{}, err
	}
	return r.HGDetail(ctx, user, id, "mine")
}

// HGSubmit 在商品行锁下重查可发布叶子类目并提交审核；已售商品的公开版本不改变。
func (r *HGRepository) HGSubmit(ctx context.Context, user, id string, version uint64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	p, err := hgOwnerState(tx.QueryRowContext(ctx, q.HGCommerceOwnerLockSQL, id, user), id)
	if err != nil {
		return err
	}
	// 本人同一商品及期望版本的提交重放，在首次成功后返回成功；不同版本仍是冲突。
	if p.AuditStatus == "PENDING" && p.Version > 0 && p.Version-1 == version {
		return nil
	}
	// 已驳回版本含审核人记录，不得直接重审；商家先编辑生成新版本。
	if p.Version != version || p.SaleStatus == "DELETED" || p.AuditStatus != "DRAFT" {
		return ErrHGConflict
	}
	if err = hgValidateCategory(ctx, tx, p.CategoryID); err != nil {
		return err
	}
	if err = hgExactlyOne(tx.ExecContext(ctx, q.HGCommerceDraftStatusSQL, id, version)); err != nil {
		return err
	}
	return tx.Commit()
}

// HGReview 先锁商品，再记录审核人及结果；线上版本只在批准时原子切换。
func (r *HGRepository) HGReview(ctx context.Context, user, id string, approve bool, reason string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var p HGProduct
	var owner string
	var category string
	err = tx.QueryRowContext(ctx, q.HGCommerceReviewLockSQL, id).Scan(&p.AuditStatus, &p.SaleStatus, &p.Version, &p.DraftVersion, &p.PublishedVersion, &owner, &category)
	if errors.Is(err, sql.ErrNoRows) {
		return sql.ErrNoRows
	}
	if err != nil {
		return err
	}
	if owner == user {
		return ErrHGForbidden
	}
	if p.AuditStatus != "PENDING" || p.SaleStatus == "DELETED" {
		return ErrHGConflict
	}
	status := "REJECTED"
	if approve {
		if err = hgValidateCategory(ctx, tx, category); err != nil {
			return err
		}
		// 新审核 SKU 只登记零可售库存；跨版本相同编码保留原库存及已预占量。
		if _, err = tx.ExecContext(ctx, q.HGCommerceInventoryEnsureSQL, id, p.DraftVersion); err != nil {
			return err
		}
		status = "APPROVED"
		p.PublishedVersion = p.DraftVersion
	}
	if err = hgExactlyOne(tx.ExecContext(ctx, q.HGCommerceReviewVersionSQL, user, reason, id, p.DraftVersion)); err != nil {
		return err
	}
	if err = hgExactlyOne(tx.ExecContext(ctx, q.HGCommerceProductStateSQL, status, p.SaleStatus, p.DraftVersion, p.PublishedVersion, id, p.Version)); err != nil {
		return err
	}
	return tx.Commit()
}

// HGSetSale 只切换销售状态，不改历史审核与订单快照；未接库存交易时上架仅供浏览。
func (r *HGRepository) HGSetSale(ctx context.Context, user, id string, version uint64, publish bool) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	p, err := hgOwnerState(tx.QueryRowContext(ctx, q.HGCommerceOwnerLockSQL, id, user), id)
	if err != nil {
		return err
	}
	if p.Version != version || p.SaleStatus == "DELETED" {
		return ErrHGConflict
	}
	status := "OFF_SHELF"
	if publish {
		if p.AuditStatus != "APPROVED" || p.PublishedVersion != p.DraftVersion || p.SaleStatus != "OFF_SHELF" {
			return ErrHGConflict
		}
		if err = hgValidateCategory(ctx, tx, p.CategoryID); err != nil {
			return err
		}
		status = "ON_SHELF"
	} else if p.SaleStatus != "ON_SHELF" {
		return ErrHGConflict
	}
	if err = hgExactlyOne(tx.ExecContext(ctx, q.HGCommerceProductStateSQL, p.AuditStatus, status, p.DraftVersion, p.PublishedVersion, id, version)); err != nil {
		return err
	}
	return tx.Commit()
}

// HGArchive 在店铺与商品行锁下归档；待审内容必须先完成裁决，归档后所有写操作拒绝。
func (r *HGRepository) HGArchive(ctx context.Context, user, id string, version uint64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	p, err := hgOwnerState(tx.QueryRowContext(ctx, q.HGCommerceOwnerLockSQL, id, user), id)
	if err != nil {
		return err
	}
	if p.Version != version || p.SaleStatus == "DELETED" || p.AuditStatus == "PENDING" {
		return ErrHGConflict
	}
	if err = hgExactlyOne(tx.ExecContext(ctx, q.HGCommerceProductStateSQL, p.AuditStatus, "DELETED", p.DraftVersion, p.PublishedVersion, id, version)); err != nil {
		return err
	}
	return tx.Commit()
}

func hgScanPage(row interface{ Scan(...any) error }) (HGProduct, error) {
	var p HGProduct
	var images []byte
	var price sql.NullInt64
	err := row.Scan(&p.ID, &p.ProductID, &p.MerchantID, &p.CategoryID, &p.AuditStatus, &p.SaleStatus, &p.Version, &p.DraftVersion, &p.PublishedVersion, &p.Title, &images, &price)
	if err != nil {
		return p, err
	}
	p.Currency = "CNY"
	p.ProductType = "PHYSICAL"
	p.MinPriceCent = price.Int64
	if err = json.Unmarshal(images, &p.ImageURLs); err != nil {
		return p, err
	}
	return p, nil
}

// HGList 每次只查一次有界页，SKU 最低价经 (product_id,content_version,id) 索引，避免逐商品往返。
func (r *HGRepository) HGList(ctx context.Context, user string, cursor uint64, scope string) (HGPage, error) {
	query := q.HGCommercePublicPageSQL
	args := []any{cursor}
	if scope == "mine" {
		query = q.HGCommerceMyPageSQL
		args = []any{user, cursor}
	} else if scope == "review" {
		query = q.HGCommerceReviewPageSQL
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return HGPage{}, err
	}
	defer rows.Close()
	items := make([]HGProduct, 0, 21)
	for rows.Next() {
		item, scanErr := hgScanPage(rows)
		if scanErr != nil {
			return HGPage{}, scanErr
		}
		if scope == "public" {
			item.AuditStatus = "APPROVED"
			item.DraftVersion = item.PublishedVersion
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return HGPage{}, err
	}
	return hgPage(items), nil
}

// HGDetail 的公开分支仅按已上架商品唯一 ID 读取已发布版本，不暴露待审资料。
func (r *HGRepository) HGDetail(ctx context.Context, user, id, scope string) (HGProduct, error) {
	var p HGProduct
	var err error
	version := uint64(0)
	if scope == "public" {
		p, err = hgProductState(r.db.QueryRowContext(ctx, q.HGCommercePublicOneSQL, id))
		version = p.PublishedVersion
	} else if scope == "mine" {
		p, err = hgOwnerState(r.db.QueryRowContext(ctx, q.HGCommerceOwnerSQL, id, user), id)
		version = p.DraftVersion
	} else {
		p, err = hgProductState(r.db.QueryRowContext(ctx, q.HGCommerceReviewDetailSQL, id))
		version = p.DraftVersion
	}
	if err != nil {
		return HGProduct{}, err
	}
	if scope == "public" {
		p.AuditStatus = "APPROVED"
		p.DraftVersion = p.PublishedVersion
	}
	var images, description, subtitle, title, reason string
	err = r.db.QueryRowContext(ctx, q.HGCommerceVersionSQL, id, version).Scan(&title, &subtitle, &description, &images, &reason)
	if err != nil {
		return HGProduct{}, err
	}
	p.Title = title
	p.Subtitle = subtitle
	p.Description = description
	if scope == "mine" {
		p.RejectReason = reason
	}
	if err = json.Unmarshal([]byte(images), &p.ImageURLs); err != nil {
		return HGProduct{}, err
	}
	rows, err := r.db.QueryContext(ctx, q.HGCommerceSKUsSQL, id, version)
	if err != nil {
		return HGProduct{}, err
	}
	defer rows.Close()
	p.SKUs = make([]HGSKU, 0, 4)
	for rows.Next() {
		var sku HGSKU
		var specs []byte
		if err = rows.Scan(&sku.SKUID, &sku.SKUCode, &specs, &sku.PriceCent, &sku.InitialStock); err != nil {
			return HGProduct{}, err
		}
		if err = json.Unmarshal(specs, &sku.Specs); err != nil {
			return HGProduct{}, err
		}
		p.SKUs = append(p.SKUs, sku)
	}
	if err = rows.Err(); err != nil {
		return HGProduct{}, err
	}
	if len(p.SKUs) > 200 {
		return HGProduct{}, ErrHGConflict
	}
	for _, sku := range p.SKUs {
		if p.MinPriceCent == 0 || sku.PriceCent < p.MinPriceCent {
			p.MinPriceCent = sku.PriceCent
		}
	}
	return p, nil
}
