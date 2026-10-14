package repository

import (
	"MLC_GO/internal/modules/recharge/model"
	hgqueries "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
)

// HGRepository 只读取 payment_recharge_order，不访问资产或关联交易。
type HGRepository struct{ hgDB *sql.DB } // 复用主服务连接池。

// HGNewRepository 组装只读仓储，不连接外部服务。
func HGNewRepository(hgDB *sql.DB) *HGRepository { return &HGRepository{hgDB: hgDB} }

// HGList 按当前用户读取最多 limit+1 条记录，limit 范围 1 至 50。
func (hgRepo *HGRepository) HGList(hgCtx context.Context, hgUser string, hgCursor *model.HGCursor, hgLimit int) ([]model.HGRecord, error) {
	if hgUser == "" || hgLimit < 1 || hgLimit > 50 {
		return nil, model.HGErrInvalid
	}
	hgSQL := hgqueries.HGRechargeFirstPageSQL
	hgArgs := []any{hgUser, hgUser}
	if hgCursor != nil {
		hgSQL = hgqueries.HGRechargeNextPageSQL
		hgArgs = append(hgArgs, hgCursor.HGCreatedAt, hgCursor.HGCreatedAt, hgCursor.HGID)
	}
	hgArgs = append(hgArgs, hgLimit+1)
	hgRows, hgErr := hgRepo.hgDB.QueryContext(hgCtx, hgSQL, hgArgs...)
	if hgErr != nil {
		return nil, hgErr
	}
	defer hgRows.Close()
	hgItems := make([]model.HGRecord, 0, hgLimit+1)
	for hgRows.Next() {
		var hgItem model.HGRecord
		var hgPaid sql.NullTime
		if hgErr = hgRows.Scan(&hgItem.HGID, &hgItem.HGOrderID, &hgItem.HGTitle, &hgItem.HGCurrency, &hgItem.HGPayAmount, &hgItem.HGTotalCoin, &hgItem.HGStatus, &hgItem.HGPaymentMode, &hgItem.HGCreatedAt, &hgItem.HGExpiresAt, &hgPaid); hgErr != nil {
			return nil, hgErr
		}
		if hgPaid.Valid {
			hgItem.HGPaidAt = &hgPaid.Time
		}
		hgItems = append(hgItems, hgItem)
	}
	return hgItems, hgRows.Err()
}
