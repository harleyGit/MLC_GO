package OpsRepositoryPackage

import (
	OpsModelPackage "MLC_GO/internal/modules/ops/model"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/go-sql-driver/mysql"
)

var ErrHGRechargeSKUConflict = errors.New("recharge SKU conflict")

// HGCreateRechargeSKU inserts one catalog row; unique codes remain reserved after deletion.
func (r *Repository) HGCreateRechargeSKU(ctx context.Context, operator string, item OpsModelPackage.HGPaymentRechargeSKU) error {
	_, err := r.db.ExecContext(ctx, SQLQueriesPackage.InsertOpsPaymentRechargeSKUSQL, item.SKUID, item.SKUCode, item.Title, item.Currency, item.PayAmount, item.CoinAmount, item.BonusCoin, item.TotalCoin, item.Status, item.SortOrder, item.StartTime, item.EndTime, operator, operator)
	return hgRechargeSKUDBError(err)
}

// HGUpdateRechargeSKU performs a single version-guarded mutation without touching historical consumers.
func (r *Repository) HGUpdateRechargeSKU(ctx context.Context, operator string, item OpsModelPackage.HGPaymentRechargeSKU) error {
	result, err := r.db.ExecContext(ctx, SQLQueriesPackage.UpdateOpsPaymentRechargeSKUSQL, item.SKUCode, item.Title, item.PayAmount, item.CoinAmount, item.BonusCoin, item.TotalCoin, item.Status, item.SortOrder, item.StartTime, item.EndTime, operator, item.SKUID, item.Version)
	return hgRechargeSKUMutationResult(result, err)
}

// HGDeleteRechargeSKU soft deletes a unique business ID using its expected version.
func (r *Repository) HGDeleteRechargeSKU(ctx context.Context, operator, id string, version uint32) error {
	result, err := r.db.ExecContext(ctx, SQLQueriesPackage.DeleteOpsPaymentRechargeSKUSQL, operator, id, version)
	return hgRechargeSKUMutationResult(result, err)
}

func hgRechargeSKUDBError(err error) error {
	var duplicate *mysql.MySQLError
	// As 是判断 err 是否可以转换为指定类型的错误，如果可以，则将其赋值给 duplicate 变量。1062 就是 唯一键值/唯一索引冲突的表示
	if errors.As(err, &duplicate) && duplicate.Number == 1062 {
		return ErrHGRechargeSKUConflict
	}
	return err
}

func hgRechargeSKUMutationResult(result sql.Result, err error) error {
	if err != nil {
		return hgRechargeSKUDBError(err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrHGRechargeSKUConflict
	}
	return nil
}

func hgScanRechargeSKU(row interface{ Scan(...any) error }) (OpsModelPackage.HGPaymentRechargeSKU, error) {
	var item OpsModelPackage.HGPaymentRechargeSKU
	var end sql.NullTime
	err := row.Scan(&item.ID, &item.SKUID, &item.SKUCode, &item.Title, &item.Currency, &item.PayAmount, &item.CoinAmount, &item.BonusCoin, &item.TotalCoin, &item.Status, &item.SortOrder, &item.StartTime, &end, &item.Version, &item.CreatedAt, &item.UpdatedAt)
	if end.Valid {
		item.EndTime = &end.Time
	}
	return item, err
}

// HGGetRechargeSKU uses the unique business ID, never an unbounded catalog scan.
func (r *Repository) HGGetRechargeSKU(ctx context.Context, id string) (OpsModelPackage.HGPaymentRechargeSKU, error) {
	item, err := hgScanRechargeSKU(r.db.QueryRowContext(ctx, SQLQueriesPackage.SelectOpsPaymentRechargeSKUBySKUIDSQL, id))
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrHGRechargeSKUConflict
	}
	return item, err
}

// HGListRechargeSKUs bounds reads to limit+1 through the (is_deleted,id) index.
func (r *Repository) HGListRechargeSKUs(ctx context.Context, cursor uint64, limit int) ([]OpsModelPackage.HGPaymentRechargeSKU, bool, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	query, args := SQLQueriesPackage.SelectOpsPaymentRechargeSKUListFirstSQL, []any{limit + 1}
	if cursor > 0 {
		query, args = SQLQueriesPackage.SelectOpsPaymentRechargeSKUListByCursorSQL, []any{cursor, limit + 1}
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	items := make([]OpsModelPackage.HGPaymentRechargeSKU, 0, limit+1)
	for rows.Next() {
		item, err := hgScanRechargeSKU(rows)
		if err != nil {
			return nil, false, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	return items, more, nil
}

// HGRechargeSKUTimeout bounds the entire authorization and catalog operation.
const HGRechargeSKUTimeout = 5 * time.Second
