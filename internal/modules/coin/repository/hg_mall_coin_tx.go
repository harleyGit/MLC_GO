package CoinRepositoryPackage

import (
	"MLC_GO/internal/events"
	CoinEventsPackage "MLC_GO/internal/events/coin"
	CoinModelPackage "MLC_GO/internal/modules/coin/model"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
)

// HGMaxMallMutationAmount 限制单笔实物订单金额；扣款仍只锁定至多1000个有效lot，无法覆盖全额时回滚整个订单。
const HGMaxMallMutationAmount uint64 = 100000

// HGMarketTx 在调用方的同一 MySQL 事务内记账；调用方负责订单/库存锁、Commit 和失败回滚。
// 只接受固定业务动作与订单标识，防止普通交易端点滥用任意资产操作。
func (r *HGRepository) HGMarketTx(ctx context.Context, tx *sql.Tx, action, userID, orderID string, amount, reference uint64) (CoinModelPackage.HGMutationResult, error) {
	if tx == nil || r == nil || r.topic == "" || userID == "" || orderID == "" || len(orderID) > 48 || amount == 0 || amount > HGMaxMallMutationAmount {
		return CoinModelPackage.HGMutationResult{}, ErrHGBusinessLimit
	}
	command := CoinModelPackage.HGCommand{UserID: userID, Amount: amount, BusinessType: "mall_order", BusinessKey: orderID}
	switch action {
	case "debit":
		command.Operation, command.RequestID, command.Reason = CoinModelPackage.HGOperationDebit, "mall:debit:"+orderID, "mall_purchase"
	case "refund":
		if reference == 0 {
			return CoinModelPackage.HGMutationResult{}, ErrHGRefundExceedsDebit
		}
		command.Operation, command.RequestID, command.Reason = CoinModelPackage.HGOperationRefund, "mall:refund:"+orderID, "mall_unshipped_refund"
		command.ReferenceTransactionID = reference
	case "settle":
		command.Operation, command.RequestID, command.Reason = CoinModelPackage.HGOperationGrant, "mall:settle:"+orderID, "mall_received_settlement"
	default:
		return CoinModelPackage.HGMutationResult{}, ErrHGBusinessLimit
	}
	if _, err := tx.ExecContext(ctx, SQLQueriesPackage.EnsureCoinWalletSQL, userID); err != nil {
		return CoinModelPackage.HGMutationResult{}, err
	}
	command.Event = CoinEventsPackage.HGAssetChangedEvent{EventMeta: events.NewEventMeta(ctx), UserID: userID, Operation: string(command.Operation), Amount: amount, BusinessType: command.BusinessType, BusinessKey: orderID, ReferenceTransactionID: reference}
	result, err := r.hgMutateTx(ctx, tx, command)
	result.Committed = false
	return result, err
}
