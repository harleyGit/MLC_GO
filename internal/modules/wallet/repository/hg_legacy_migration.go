package repository

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"regexp"

	"MLC_GO/internal/modules/wallet/model"
	hgqueries "MLC_GO/internal/pkg/mysql/queries"
)

// HGMigrateLegacyRange 仅供停写后的手动迁移工具使用，闭区间最多500个主键。
// 不调用任何资产能力；提交结果未知时重跑同一区间，完整比对已迁移快照后跳过。
func (hgRepo *HGRepository) HGMigrateLegacyRange(hgCtx context.Context, hgLow, hgHigh uint64) (int, error) {
	if hgLow == 0 || hgHigh < hgLow || hgHigh-hgLow >= 500 {
		return 0, model.HGErrInvalid
	}
	hgTx, hgErr := hgRepo.hgDB.BeginTx(hgCtx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if hgErr != nil {
		return 0, hgErr
	}
	defer hgTx.Rollback()
	hgRows, hgErr := hgTx.QueryContext(hgCtx, hgqueries.HGWalletLegacyRangeSQL, hgLow, hgHigh)
	if hgErr != nil {
		return 0, hgErr
	}
	defer hgRows.Close()
	hgOrders := make([]model.HGOrder, 0, 500)
	hgIdentifier := regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	for hgRows.Next() {
		hgOrder, hgScanErr := hgScanOrder(hgRows)
		if hgScanErr != nil {
			return 0, hgScanErr
		}
		if !hgIdentifier.MatchString(hgOrder.HGSKUID) || !hgIdentifier.MatchString(hgOrder.HGOrderID) || !hgIdentifier.MatchString(hgOrder.HGRequestID) || hgOrder.HGCurrency != "CNY" || hgOrder.HGPayAmount == 0 || hgOrder.HGPayAmount > 9007199254740991 || hgOrder.HGTotalCoin == 0 || hgOrder.HGTotalCoin > 100000 || (hgOrder.HGPaymentMode != "unavailable" && hgOrder.HGPaymentMode != "platform_debug") {
			return 0, model.HGErrConflict
		}
		if hgOrder.HGStatus == "paid" {
			if hgOrder.HGPaymentMode != "platform_debug" || hgOrder.HGPaidAt == nil || hgOrder.HGPaymentTransactionID == 0 {
				return 0, model.HGErrConflict
			}
		} else if hgOrder.HGStatus != "pending" || hgOrder.HGPaidAt != nil || hgOrder.HGPaymentTransactionID != 0 || hgOrder.HGBalanceAfter != 0 {
			return 0, model.HGErrConflict
		}
		hgOrders = append(hgOrders, hgOrder)
	}
	if hgErr = hgRows.Err(); hgErr != nil {
		return 0, hgErr
	}
	if hgErr = hgRows.Close(); hgErr != nil {
		return 0, hgErr
	}
	for _, hgOrder := range hgOrders {
		hgExisting, hgReadErr := hgScanOrder(hgTx.QueryRowContext(hgCtx, hgqueries.HGWalletLegacyByOwnerSQL, hgOrder.HGOrderID, hgOrder.HGUserID))
		if hgReadErr == nil {
			if !reflect.DeepEqual(hgExisting, hgOrder) {
				return 0, model.HGErrConflict
			}
			continue
		}
		if !errors.Is(hgReadErr, sql.ErrNoRows) {
			return 0, hgReadErr
		}
		hgHash := hgRequestHash(hgOrder.HGSKUID)
		if _, hgErr = hgTx.ExecContext(hgCtx, hgqueries.HGWalletInsertTradeSQL, hgOrder.HGOrderID, hgOrder.HGUserID, hgOrder.HGRequestID, hgHash, hgOrder.HGTitle, hgOrder.HGCurrency, hgOrder.HGPayAmount, hgOrder.HGPayAmount, hgOrder.HGExpiresAt, hgOrder.HGCreatedAt); hgErr != nil {
			return 0, hgErr
		}
		hgStatus := 1
		var hgTransaction, hgBalance any
		if hgOrder.HGStatus == "paid" {
			hgStatus, hgTransaction, hgBalance = 3, hgOrder.HGPaymentTransactionID, hgOrder.HGBalanceAfter
			if _, hgErr = hgTx.ExecContext(hgCtx, hgqueries.HGWalletCompleteTradeSQL, hgOrder.HGPaidAt, hgOrder.HGPaidAt, hgOrder.HGOrderID, hgOrder.HGUserID); hgErr != nil {
				return 0, hgErr
			}
		}
		if _, hgErr = hgTx.ExecContext(hgCtx, hgqueries.HGWalletInsertLegacyOrderSQL, hgOrder.HGOrderID, hgOrder.HGOrderID, hgOrder.HGUserID, hgOrder.HGRequestID, hgHash, hgOrder.HGSKUID, hgOrder.HGDisplayName, hgOrder.HGTitle, hgOrder.HGDescription, hgOrder.HGCurrency, hgOrder.HGPayAmount, hgOrder.HGTotalCoin, hgOrder.HGCreatedAt, hgOrder.HGExpiresAt, hgOrder.HGPaymentMode, hgStatus, hgOrder.HGPaidAt, hgOrder.HGPaidAt, hgTransaction, hgBalance); hgErr != nil {
			return 0, hgErr
		}
	}
	if hgErr = hgTx.Commit(); hgErr != nil {
		return 0, hgErr
	}
	return len(hgOrders), nil
}
