package repository

import (
	hgcoinrepo "MLC_GO/internal/modules/coin/repository"
	hgcoin "MLC_GO/internal/modules/coin/service"
	"MLC_GO/internal/modules/wallet/model"
	hgconfig "MLC_GO/internal/pkg/config"
	hgqueries "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"
)

// HGRepository 负责目录、订单及debug支付和coin同事务编排。
type HGRepository struct{ hgDB *sql.DB }

// HGNewRepository 复用主服务MySQL连接池，不发起连接或迁移。
func HGNewRepository(hgDB *sql.DB) *HGRepository { return &HGRepository{hgDB: hgDB} }

// hgScanSKU
//
//	@param hgRow 参数只要求“具有 Scan 方法”即可，不关心它具体是什么类型。便于解耦。
//	@return model.HGSKU
//	@return error
func hgScanSKU(hgRow interface{ Scan(...any) error }) (model.HGSKU, error) {
	var hgItem model.HGSKU
	var hgEnd sql.NullTime
	hgErr := hgRow.Scan(&hgItem.HGID, &hgItem.HGSKUID, &hgItem.HGTitle, &hgItem.HGCurrency, &hgItem.HGPayAmount, &hgItem.HGCoinAmount, &hgItem.HGBonusCoin, &hgItem.HGTotalCoin, &hgItem.HGStartTime, &hgEnd)
	if hgEnd.Valid {
		hgItem.HGEndTime = &hgEnd.Time
	}
	return hgItem, hgErr
}

// HGListCandidates 最多读取201条候选，避免无有效目录时扫描整表。
func (hgRepo *HGRepository) HGListCandidates(hgCtx context.Context, hgCursor uint64) ([]model.HGSKU, error) {
	// 通过cusor多查一条，比如：前端传来200，实际查询201条数据，实际返回200条。多出来的一条数据就是用来看看是不是isMore
	hgRows, hgErr := hgRepo.hgDB.QueryContext(hgCtx, hgqueries.HGWalletSKUWindowSQL, hgCursor)
	if hgErr != nil {
		return nil, hgErr
	}
	defer hgRows.Close()
	hgItems := make([]model.HGSKU, 0, 201)
	for hgRows.Next() {
		hgItem, hgErr := hgScanSKU(hgRows)
		if hgErr != nil {
			return nil, hgErr
		}
		hgItems = append(hgItems, hgItem)
	}
	return hgItems, hgRows.Err()
}

func hgScanOrder(hgRow interface{ Scan(...any) error }) (model.HGOrder, error) {
	var hgItem model.HGOrder
	var hgPaidAt sql.NullTime
	var hgTransactionID, hgBalanceAfter sql.NullInt64
	hgErr := hgRow.Scan(&hgItem.HGOrderID, &hgItem.HGUserID, &hgItem.HGRequestID, &hgItem.HGSKUID, &hgItem.HGDisplayName, &hgItem.HGTitle, &hgItem.HGDescription, &hgItem.HGCurrency, &hgItem.HGPayAmount, &hgItem.HGTotalCoin, &hgItem.HGCreatedAt, &hgItem.HGExpiresAt, &hgItem.HGPaymentMode, &hgItem.HGStatus, &hgPaidAt, &hgTransactionID, &hgBalanceAfter)
	if hgPaidAt.Valid {
		hgItem.HGPaidAt = &hgPaidAt.Time
	}
	if hgTransactionID.Valid {
		hgItem.HGPaymentTransactionID = uint64(hgTransactionID.Int64)
	}
	if hgBalanceAfter.Valid {
		hgItem.HGBalanceAfter = uint64(hgBalanceAfter.Int64)
	}
	return hgItem, hgErr
}

// HGDetail 在SQL中限定owner；不存在和他人订单返回相同错误。
func (hgRepo *HGRepository) HGDetail(hgCtx context.Context, hgUser, hgID string) (model.HGOrder, error) {
	hgItem, hgErr := hgScanOrder(hgRepo.hgDB.QueryRowContext(hgCtx, hgqueries.HGWalletOrderByOwnerSQL, hgID, hgUser))
	if errors.Is(hgErr, sql.ErrNoRows) {
		hgErr = model.HGErrNotFound
	}
	return hgItem, hgErr
}

func (hgRepo *HGRepository) hgReplay(hgCtx context.Context, hgUser, hgRequest, hgSKU string) (model.HGOrder, error) {
	hgItem, hgErr := hgScanOrder(hgRepo.hgDB.QueryRowContext(hgCtx, hgqueries.HGWalletOrderByRequestSQL, hgUser, hgRequest))
	if hgErr == nil && hgItem.HGSKUID != hgSKU {
		return model.HGOrder{}, model.HGErrConflict
	}
	return hgItem, hgErr
}

// 创建一笔充值订单，同时防止同一个 requestId 被重复创建。
/*
① 先查有没有已经创建过
        ↓
② 没有 → 开启数据库事务
        ↓
③ 查询 SKU，并检查 SKU 是否有效
        ↓
④ 生成订单号
        ↓
⑤ 查询用户昵称
        ↓
⑥ 插入订单
        ↓
⑦ 如果发现重复订单 → 返回之前的订单
        ↓
⑧ 插入成功 → Commit 提交事务
*/
// HGCreate 以短事务锁定目录并写不可变快照；重复键回滚后重新点查已提交订单。
// 幂等记录不清理，过期重放不续期；提交结果未知时由调用方用同requestId重试确认。
//	@param hgCtx
//	@param hgUser
//	@param hgSKU
//	@param hgRequest
//	@param hgNow
//	@return model.HGOrder
//	@return error
func (hgRepo *HGRepository) HGCreate(hgCtx context.Context, hgUser, hgSKU, hgRequest string, hgNow time.Time) (model.HGOrder, error) {
	// 先根据 user + requestId + sku 查订单； 找到了 → 直接返回原订单； 没有找到继续下面流程，进入创建订单流程
	hgExisting, hgErr := hgRepo.hgReplay(hgCtx, hgUser, hgRequest, hgSKU)
	if !errors.Is(hgErr, sql.ErrNoRows) {
		return hgExisting, hgErr
	}
	var hgRandom [32]byte
	// 生成 32 字节随机数据。 
	if _, hgErr = rand.Read(hgRandom[:]); hgErr != nil {
		return model.HGOrder{}, hgErr
	}

	// 开启一个数据库事务，并指定事务的隔离级别为 Read Committed。在事务中查询数据时，只能看到其他事务已经提交的数据。后面的查询、插入订单都放入这个事务里，中途出错就回滚。
	hgTx, hgErr := hgRepo.hgDB.BeginTx(hgCtx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if hgErr != nil {
		return model.HGOrder{}, hgErr
	}
	defer hgTx.Rollback()
	// 查询SKU
	hgSKUItem, hgErr := hgScanSKU(hgTx.QueryRowContext(hgCtx, hgqueries.HGWalletSKUForShareSQL, hgSKU, hgNow, hgNow))
	if errors.Is(hgErr, sql.ErrNoRows) {
		_ = hgTx.Rollback()
		// 同requestId的并发创建可能已提交，随后目录被停用；优先恢复原始快照。
		hgExisting, hgErr = hgRepo.hgReplay(hgCtx, hgUser, hgRequest, hgSKU)
		if errors.Is(hgErr, sql.ErrNoRows) {
			hgErr = model.HGErrSKUUnavailable
		}
		return hgExisting, hgErr
	}
	if hgErr != nil {
		return model.HGOrder{}, hgErr
	}
	if hgSKUItem.HGTotalCoin > hgcoin.HGMaxMutationAmount {
		return model.HGOrder{}, model.HGErrUnsupported
	}
	// 检测SKU是否合法
	if hgSKUItem.HGSKUID != hgSKU || hgSKUItem.HGCurrency != "CNY" || hgSKUItem.HGPayAmount == 0 || hgSKUItem.HGPayAmount > 9007199254740991 || hgSKUItem.HGTotalCoin == 0 || hgSKUItem.HGCoinAmount == 0 || hgSKUItem.HGCoinAmount > hgSKUItem.HGTotalCoin || hgSKUItem.HGBonusCoin != hgSKUItem.HGTotalCoin-hgSKUItem.HGCoinAmount {
		return model.HGOrder{}, model.HGErrSKUUnavailable
	}
	// hex.EncodeToString(hgRandom[:]) 把它变成字符串订单ID
	// 把用户、SKU、金额、平台币数量、创建时间、过期时间等组装成一个订单。
	hgOrder := model.HGOrder{HGOrderID: hex.EncodeToString(hgRandom[:]), HGUserID: hgUser, HGRequestID: hgRequest, HGSKUID: hgSKUItem.HGSKUID, HGTitle: hgSKUItem.HGTitle, HGDescription: fmt.Sprintf("充值获得%d平台币", hgSKUItem.HGTotalCoin), HGCurrency: hgSKUItem.HGCurrency, HGPayAmount: hgSKUItem.HGPayAmount, HGTotalCoin: hgSKUItem.HGTotalCoin, HGCreatedAt: hgNow, HGExpiresAt: hgNow.Add(10 * time.Minute)}
	hgOrder.HGPaymentMode, hgOrder.HGStatus = "unavailable", "pending"
	if hgconfig.IsWalletDebugPaymentEnabled() {
		hgOrder.HGPaymentMode = "platform_debug"
	}
	if hgErr = hgTx.QueryRowContext(hgCtx, hgqueries.HGWalletDisplayNameSQL, hgUser).Scan(&hgOrder.HGDisplayName); hgErr != nil {
		return model.HGOrder{}, hgErr
	}
	// 执行 INSERT，把订单保存到数据库。
	_, hgErr = hgTx.ExecContext(hgCtx, hgqueries.HGWalletInsertOrderSQL, hgOrder.HGOrderID, hgUser, hgRequest, hgOrder.HGSKUID, hgOrder.HGDisplayName, hgOrder.HGTitle, hgOrder.HGDescription, hgOrder.HGCurrency, hgOrder.HGPayAmount, hgOrder.HGTotalCoin, hgOrder.HGCreatedAt, hgOrder.HGExpiresAt, hgOrder.HGPaymentMode)
	if hgErr != nil {
		var hgDuplicate *mysql.MySQLError
		// 1062 是 MySQL 唯一键冲突。
		// 两个请求几乎同时创建了同一个 requestId，其中一个已经成功插入，另一个再插入就冲突了。
		// 直接把已经存在的订单返回。这就是典型的幂等处理。
		if errors.As(hgErr, &hgDuplicate) && hgDuplicate.Number == 1062 {
			_ = hgTx.Rollback()
			return hgRepo.hgReplay(hgCtx, hgUser, hgRequest, hgSKU)
		}
		return model.HGOrder{}, hgErr
	}
	if hgErr = hgTx.Commit(); hgErr != nil {
		return model.HGOrder{}, hgErr
	}
	return hgOrder, nil
}

// HGPayDebug 锁定本人订单，再在同一短事务入账；失败由调用方按订单重试/查询，不另开资产事务。
func (hgRepo *HGRepository) HGPayDebug(hgCtx context.Context, hgUser, hgOrderID string) (model.HGOrder, error) {
	hgTx, hgErr := hgRepo.hgDB.BeginTx(hgCtx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if hgErr != nil {
		return model.HGOrder{}, hgErr
	}
	defer hgTx.Rollback()
	// 根据 订单ID + 用户ID 找到订单。
	hgOrder, hgErr := hgScanOrder(hgTx.QueryRowContext(hgCtx, hgqueries.HGWalletOrderForPaySQL, hgOrderID, hgUser))
	if errors.Is(hgErr, sql.ErrNoRows) {
		return model.HGOrder{}, model.HGErrNotFound
	}
	if hgErr != nil {
		return model.HGOrder{}, hgErr
	}
	// 检查订单能不能支付
	if hgOrder.HGPaymentMode != "platform_debug" {
		return model.HGOrder{}, model.HGErrPaymentUnavailable
	}
	// 必须是调试支付模式。如果已经支付：直接返回，不重复充值。
	if hgOrder.HGStatus == "paid" {
		return hgOrder, nil
	}
	// 调试支付功能开启，并且订单状态必须是 pending。
	if !hgconfig.IsWalletDebugPaymentEnabled() || hgOrder.HGStatus != "pending" {
		return model.HGOrder{}, model.HGErrPaymentUnavailable
	}
	// 当前时间已经超过订单过期时间 → 不能支付。
	hgNow := time.Now().UTC().Truncate(time.Millisecond)
	if !hgOrder.HGExpiresAt.After(hgNow) {
		return model.HGOrder{}, model.HGErrOrderExpired
	}
	if hgOrder.HGTotalCoin == 0 || hgOrder.HGTotalCoin > hgcoin.HGMaxMutationAmount {
		return model.HGOrder{}, model.HGErrUnsupported
	}
	// 真正给用户充值：给用户增加 HGTotalCoin 个平台币。
	hgResult, hgErr := hgcoinrepo.NewHGRepository(hgRepo.hgDB, "mlc.domain.events").HGDebugRechargeTx(hgCtx, hgTx, hgUser, hgOrderID, hgOrder.HGTotalCoin)
	if hgErr != nil {
		return model.HGOrder{}, hgErr
	}
	hgUpdate, hgErr := hgTx.ExecContext(hgCtx, hgqueries.HGWalletMarkPaidSQL, hgNow, hgResult.TransactionID, hgResult.BalanceAfter, hgOrderID, hgUser)
	if hgErr != nil {
		return model.HGOrder{}, hgErr
	}
	// 我期望恰好修改 1 条订单记录。如果是 0 条或者其他数量，说明可能发生了并发修改/数据异常，因此认为冲突。
	hgCount, hgErr := hgUpdate.RowsAffected()
	if hgErr != nil {
		return model.HGOrder{}, hgErr
	}
	if hgCount != 1 {
		return model.HGOrder{}, model.HGErrConflict
	}
	if hgErr = hgTx.Commit(); hgErr != nil {
		return model.HGOrder{}, hgErr
	}
	hgOrder.HGStatus, hgOrder.HGPaidAt, hgOrder.HGPaymentTransactionID, hgOrder.HGBalanceAfter = "paid", &hgNow, hgResult.TransactionID, hgResult.BalanceAfter
	return hgOrder, nil
}
