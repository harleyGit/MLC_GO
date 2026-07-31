package CoinRepositoryPackage

import (
	"MLC_GO/internal/events"
	CoinEventsPackage "MLC_GO/internal/events/coin"
	CoinModelPackage "MLC_GO/internal/modules/coin/model"
	"MLC_GO/internal/outbox"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/go-sql-driver/mysql"
)

var (
	ErrHGIdempotencyConflict = errors.New("coin request idempotency conflict")
	ErrHGInsufficientBalance = errors.New("insufficient coin balance")
	ErrHGBusinessLimit       = errors.New("coin business limit exceeded")
	ErrHGRefundExceedsDebit  = errors.New("coin refund exceeds original debit")
	ErrHGBalanceOverflow     = errors.New("coin balance overflow")
)

type hgCoinLot struct {
	id        uint64
	remaining uint64
	expiresAt sql.NullTime
}

const hgCoinMutationMaxAttempts = 3

// HGRepository owns authoritative wallet locking, immutable transactions, lots, allocations, and outbox atomicity.
type HGRepository struct {
	db    *sql.DB
	topic string
	hgNow func() time.Time
}

func NewHGRepository(db *sql.DB, topic string) *HGRepository {
	return &HGRepository{db: db, topic: topic, hgNow: time.Now}
}

// Balance lazily initializes a missing wallet and returns its authoritative value.
func (r *HGRepository) Balance(ctx context.Context, userID string) (uint64, error) {
	if err := r.EnsureWallet(ctx, userID); err != nil {
		return 0, err
	}
	var balance uint64
	if err := r.db.QueryRowContext(ctx, SQLQueriesPackage.SelectCoinWalletSQL, userID).Scan(&balance); err != nil {
		return 0, fmt.Errorf("select coin wallet: %w", err)
	}
	return balance, nil
}

// ListTransactions 按用户和复合 keyset 游标读取有界审计流水；limit 由调用方限制，Repository 再执行防御性收敛。
func (r *HGRepository) ListTransactions(ctx context.Context, userID string, cursor CoinModelPackage.HGTransactionCursor, limit int) ([]CoinModelPackage.HGTransaction, CoinModelPackage.HGTransactionCursor, bool, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	queryLimit := limit + 1
	querySQL := SQLQueriesPackage.SelectCoinTransactionsFirstSQL
	args := []any{userID, queryLimit}
	if !cursor.CreatedAt.IsZero() && cursor.ID > 0 {
		querySQL = SQLQueriesPackage.SelectCoinTransactionsByCursorSQL
		args = []any{userID, cursor.CreatedAt, cursor.CreatedAt, cursor.ID, queryLimit}
	}
	rows, err := r.db.QueryContext(ctx, querySQL, args...)
	if err != nil {
		return nil, CoinModelPackage.HGTransactionCursor{}, false, fmt.Errorf("list coin transactions: %w", err)
	}
	defer rows.Close()
	items := make([]CoinModelPackage.HGTransaction, 0, queryLimit)
	for rows.Next() {
		var item CoinModelPackage.HGTransaction
		var reference sql.NullInt64
		if err := rows.Scan(&item.ID, &item.UserID, &item.RequestID, &item.Operation, &item.Amount, &item.SignedDelta, &item.BalanceAfter, &item.Reason, &item.BusinessType, &item.BusinessKey, &reference, &item.CreatedAt); err != nil {
			return nil, CoinModelPackage.HGTransactionCursor{}, false, fmt.Errorf("scan coin transaction: %w", err)
		}
		if reference.Valid && reference.Int64 > 0 {
			item.ReferenceTransactionID = uint64(reference.Int64)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, CoinModelPackage.HGTransactionCursor{}, false, fmt.Errorf("iterate coin transactions: %w", err)
	}
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	next := CoinModelPackage.HGTransactionCursor{}
	if len(items) > 0 {
		last := items[len(items)-1]
		next = CoinModelPackage.HGTransactionCursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}
	return items, next, hasMore, nil
}

// EnsureWallet idempotently creates an empty wallet without generating a financial transaction or outbox event.
func (r *HGRepository) EnsureWallet(ctx context.Context, userID string) error {
	if _, err := r.db.ExecContext(ctx, SQLQueriesPackage.EnsureCoinWalletSQL, userID); err != nil {
		return fmt.Errorf("ensure coin wallet: %w", err)
	}
	return nil
}

// Initialize supports the bounded initializer without exposing mutation details to the job.
func (r *HGRepository) Initialize(ctx context.Context, command CoinModelPackage.HGCommand) (CoinModelPackage.HGMutationResult, error) {
	command.Operation = CoinModelPackage.HGOperationInitialize
	if command.Event == nil {
		command.Event = CoinEventsPackage.HGAssetChangedEvent{EventMeta: events.NewEventMeta(ctx), UserID: command.UserID, Operation: string(command.Operation)}
	}
	return r.Mutate(ctx, command)
}

// Mutate executes one idempotent asset command under the user's wallet row lock.
func (r *HGRepository) Mutate(ctx context.Context, command CoinModelPackage.HGCommand) (result CoinModelPackage.HGMutationResult, err error) {
	started := time.Now()
	defer func() { hgObserveCoinMutation(command.Operation, result.Committed, time.Since(started), err) }()
	if err = r.EnsureWallet(ctx, command.UserID); err != nil {
		return result, err
	}
	for attempt := 1; attempt <= hgCoinMutationMaxAttempts; attempt++ {
		result, err = r.hgMutateOnce(ctx, command)
		if err == nil || !hgCoinRetryableTransactionError(err) || attempt == hgCoinMutationMaxAttempts {
			return result, err
		}
		timer := time.NewTimer(time.Duration(attempt) * 10 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return CoinModelPackage.HGMutationResult{}, ctx.Err()
		case <-timer.C:
		}
	}
	return result, err
}

func (r *HGRepository) hgMutateOnce(ctx context.Context, command CoinModelPackage.HGCommand) (result CoinModelPackage.HGMutationResult, err error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return result, fmt.Errorf("begin coin mutation: %w", err)
	}
	defer tx.Rollback()

	var balance uint64
	if err := tx.QueryRowContext(ctx, SQLQueriesPackage.SelectCoinWalletForUpdateSQL, command.UserID).Scan(&balance); err != nil {
		return result, fmt.Errorf("lock coin wallet: %w", err)
	}
	legacyReplay, err := r.hgLegacyReplay(ctx, tx, command, balance)
	if err != nil {
		return result, err
	}
	if legacyReplay {
		if err := tx.Commit(); err != nil {
			return result, fmt.Errorf("commit legacy coin replay: %w", err)
		}
		return CoinModelPackage.HGMutationResult{BalanceAfter: balance}, nil
	}
	hash := hgCommandHash(command)
	insert, err := tx.ExecContext(ctx, SQLQueriesPackage.InsertCoinRequestSQL, command.UserID, command.RequestID, command.Operation, hash)
	if err != nil {
		return result, fmt.Errorf("insert coin request: %w", err)
	}
	inserted, err := insert.RowsAffected()
	if err != nil {
		return result, fmt.Errorf("read coin request result: %w", err)
	}
	if inserted == 0 {
		return r.hgReplay(ctx, tx, command, hash)
	}

	result.BalanceAfter = balance
	switch command.Operation {
	case CoinModelPackage.HGOperationInitialize:
		result, err = r.hgRecordMutation(ctx, tx, command, balance)
	case CoinModelPackage.HGOperationRecharge, CoinModelPackage.HGOperationGrant:
		result, err = r.hgCredit(ctx, tx, command, balance)
	case CoinModelPackage.HGOperationDebit:
		result, err = r.hgDebit(ctx, tx, command, balance)
	case CoinModelPackage.HGOperationRefund:
		result, err = r.hgRefund(ctx, tx, command, balance)
	case CoinModelPackage.HGOperationCorrection:
		if command.SignedDelta < 0 {
			result, err = r.hgDebit(ctx, tx, command, balance)
		} else {
			result, err = r.hgCredit(ctx, tx, command, balance)
		}
	default:
		err = fmt.Errorf("unsupported coin operation %q", command.Operation)
	}
	if err != nil {
		return CoinModelPackage.HGMutationResult{}, err
	}
	if command.Event != nil {
		event := hgCoinEventWithBalance(command.Event, result.BalanceAfter)
		if err := outbox.NewRepository(r.db, r.topic).SaveTx(ctx, tx, event); err != nil {
			return CoinModelPackage.HGMutationResult{}, fmt.Errorf("save coin outbox: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return CoinModelPackage.HGMutationResult{}, fmt.Errorf("commit coin mutation: %w", err)
	}
	result.Committed = true
	return result, nil
}

func (r *HGRepository) hgLegacyReplay(ctx context.Context, tx *sql.Tx, command CoinModelPackage.HGCommand, balance uint64) (bool, error) {
	if command.Operation != CoinModelPackage.HGOperationDebit || command.BusinessType != "video_coin" {
		return false, nil
	}
	var submissionID, status string
	var quantity uint64
	err := tx.QueryRowContext(ctx, SQLQueriesPackage.SelectLegacyCoinCommandSQL, command.UserID, command.RequestID).Scan(&submissionID, &quantity, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("select legacy coin command: %w", err)
	}
	if submissionID != command.BusinessKey || quantity != command.Amount || status != "completed" {
		return false, ErrHGIdempotencyConflict
	}
	_ = balance
	return true, nil
}

func hgCoinRetryableTransactionError(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && (mysqlErr.Number == 1213 || mysqlErr.Number == 1205)
}

func hgCoinEventWithBalance(event events.DomainEvent, balanceAfter uint64) events.DomainEvent {
	switch value := event.(type) {
	case CoinEventsPackage.HGAssetChangedEvent:
		value.BalanceAfter = balanceAfter
		return value
	case *CoinEventsPackage.HGAssetChangedEvent:
		copy := *value
		copy.BalanceAfter = balanceAfter
		return copy
	default:
		return event
	}
}

func (r *HGRepository) hgReplay(ctx context.Context, tx *sql.Tx, command CoinModelPackage.HGCommand, hash string) (CoinModelPackage.HGMutationResult, error) {
	var operation, storedHash, status string
	var transactionID sql.NullInt64
	var balanceAfter uint64
	if err := tx.QueryRowContext(ctx, SQLQueriesPackage.SelectCoinRequestSQL, command.UserID, command.RequestID).Scan(&operation, &storedHash, &status, &transactionID, &balanceAfter); err != nil {
		return CoinModelPackage.HGMutationResult{}, fmt.Errorf("select coin request: %w", err)
	}
	if operation != string(command.Operation) || storedHash != hash || status != "completed" {
		return CoinModelPackage.HGMutationResult{}, ErrHGIdempotencyConflict
	}
	if err := tx.Commit(); err != nil {
		return CoinModelPackage.HGMutationResult{}, fmt.Errorf("commit coin replay: %w", err)
	}
	return CoinModelPackage.HGMutationResult{TransactionID: uint64(transactionID.Int64), BalanceAfter: balanceAfter}, nil
}

func (r *HGRepository) hgCredit(ctx context.Context, tx *sql.Tx, command CoinModelPackage.HGCommand, balance uint64) (CoinModelPackage.HGMutationResult, error) {
	if balance > math.MaxInt64 || command.Amount > uint64(math.MaxInt64)-balance {
		return CoinModelPackage.HGMutationResult{}, ErrHGBalanceOverflow
	}
	result, err := tx.ExecContext(ctx, SQLQueriesPackage.CreditCoinWalletSQL, command.Amount, command.UserID, uint64(math.MaxInt64)-command.Amount)
	if err != nil {
		return CoinModelPackage.HGMutationResult{}, fmt.Errorf("credit coin wallet: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != 1 {
		return CoinModelPackage.HGMutationResult{}, ErrHGBalanceOverflow
	}
	mutation, err := r.hgRecordMutation(ctx, tx, command, balance+command.Amount)
	if err != nil {
		return mutation, err
	}
	if _, err := tx.ExecContext(ctx, SQLQueriesPackage.InsertCoinLotSQL, command.UserID, mutation.TransactionID, command.Amount, command.Amount, command.ExpiresAt); err != nil {
		return CoinModelPackage.HGMutationResult{}, fmt.Errorf("insert coin lot: %w", err)
	}
	return mutation, nil
}

func (r *HGRepository) hgDebit(ctx context.Context, tx *sql.Tx, command CoinModelPackage.HGCommand, balance uint64) (CoinModelPackage.HGMutationResult, error) {
	if command.BusinessLimit > 0 {
		var used uint64
		if err := tx.QueryRowContext(ctx, SQLQueriesPackage.SelectCoinBusinessDebitTotalSQL,
			command.UserID, command.BusinessType, command.BusinessKey, command.BusinessType, command.UserID, command.BusinessKey).Scan(&used); err != nil {
			return CoinModelPackage.HGMutationResult{}, fmt.Errorf("select coin business debit total: %w", err)
		}
		if used > command.BusinessLimit || command.Amount > command.BusinessLimit-used {
			return CoinModelPackage.HGMutationResult{}, ErrHGBusinessLimit
		}
	}
	if balance < command.Amount {
		return CoinModelPackage.HGMutationResult{}, ErrHGInsufficientBalance
	}
	// FEFO 查询只锁定最多 1000 个最早到期 lot；Service 同步限制单次最多消费 1000 枚，控制锁行数和事务时长。
	rows, err := tx.QueryContext(ctx, SQLQueriesPackage.SelectCoinLotsForDebitSQL, command.UserID, r.hgNow().UTC())
	if err != nil {
		return CoinModelPackage.HGMutationResult{}, fmt.Errorf("select coin lots: %w", err)
	}
	lots := make([]hgCoinLot, 0, 8)
	var available uint64
	for rows.Next() {
		var lot hgCoinLot
		if err := rows.Scan(&lot.id, &lot.remaining, &lot.expiresAt); err != nil {
			rows.Close()
			return CoinModelPackage.HGMutationResult{}, fmt.Errorf("scan coin lot: %w", err)
		}
		lots = append(lots, lot)
		if available <= math.MaxUint64-lot.remaining {
			available += lot.remaining
		}
	}
	if err := rows.Close(); err != nil {
		return CoinModelPackage.HGMutationResult{}, fmt.Errorf("close coin lots: %w", err)
	}
	if available < command.Amount {
		// migration 12 只有 wallet/ledger，没有 lot。按 wallet 与现存 lot 的差额惰性补 opening lot，
		// 同时兼容完全没有 lot 的旧用户和迁移期间已产生部分新 lot 的混合状态。
		if balance > available {
			openingAmount := balance - available
			opening := CoinModelPackage.HGCommand{Operation: CoinModelPackage.HGOperationInitialize, UserID: command.UserID, RequestID: "opening:" + command.UserID, Amount: balance, Reason: "legacy_wallet_opening"}
			opening.Amount = openingAmount
			insert, insertErr := tx.ExecContext(ctx, SQLQueriesPackage.InsertCoinRequestSQL, opening.UserID, opening.RequestID, opening.Operation, hgCommandHash(opening))
			if insertErr != nil {
				return CoinModelPackage.HGMutationResult{}, fmt.Errorf("insert opening coin request: %w", insertErr)
			}
			inserted, insertErr := insert.RowsAffected()
			if insertErr != nil {
				return CoinModelPackage.HGMutationResult{}, fmt.Errorf("read opening coin request: %w", insertErr)
			}
			if inserted == 1 {
				openingMutation, recordErr := r.hgRecordMutation(ctx, tx, opening, balance)
				if recordErr != nil {
					return CoinModelPackage.HGMutationResult{}, recordErr
				}
				if _, insertErr = tx.ExecContext(ctx, SQLQueriesPackage.InsertCoinLotSQL, opening.UserID, openingMutation.TransactionID, openingAmount, openingAmount, nil); insertErr != nil {
					return CoinModelPackage.HGMutationResult{}, fmt.Errorf("insert opening coin lot: %w", insertErr)
				}
			}
			rows, err = tx.QueryContext(ctx, SQLQueriesPackage.SelectCoinLotsForDebitSQL, command.UserID, r.hgNow().UTC())
			if err != nil {
				return CoinModelPackage.HGMutationResult{}, fmt.Errorf("select opening coin lot: %w", err)
			}
			lots = lots[:0]
			available = 0
			for rows.Next() {
				var lot hgCoinLot
				if err := rows.Scan(&lot.id, &lot.remaining, &lot.expiresAt); err != nil {
					rows.Close()
					return CoinModelPackage.HGMutationResult{}, fmt.Errorf("scan opening coin lot: %w", err)
				}
				lots = append(lots, lot)
				available += lot.remaining
			}
			if err := rows.Close(); err != nil {
				return CoinModelPackage.HGMutationResult{}, fmt.Errorf("close opening coin lots: %w", err)
			}
		}
		if available < command.Amount {
			return CoinModelPackage.HGMutationResult{}, ErrHGInsufficientBalance
		}
	}
	result, err := tx.ExecContext(ctx, SQLQueriesPackage.DebitCoinWalletSQL, command.Amount, command.UserID, command.Amount)
	if err != nil {
		return CoinModelPackage.HGMutationResult{}, fmt.Errorf("debit coin wallet: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != 1 {
		return CoinModelPackage.HGMutationResult{}, ErrHGInsufficientBalance
	}
	mutation, err := r.hgRecordMutation(ctx, tx, command, balance-command.Amount)
	if err != nil {
		return mutation, err
	}
	remaining := command.Amount
	for _, lot := range lots {
		if remaining == 0 {
			break
		}
		allocated := lot.remaining
		if allocated > remaining {
			allocated = remaining
		}
		if _, err := tx.ExecContext(ctx, SQLQueriesPackage.UpdateCoinLotRemainingSQL, lot.remaining-allocated, lot.id); err != nil {
			return CoinModelPackage.HGMutationResult{}, fmt.Errorf("update coin lot: %w", err)
		}
		if _, err := tx.ExecContext(ctx, SQLQueriesPackage.InsertCoinAllocationSQL, mutation.TransactionID, lot.id, allocated, "debit"); err != nil {
			return CoinModelPackage.HGMutationResult{}, fmt.Errorf("insert coin allocation: %w", err)
		}
		remaining -= allocated
	}
	return mutation, nil
}

func (r *HGRepository) hgRefund(ctx context.Context, tx *sql.Tx, command CoinModelPackage.HGCommand, balance uint64) (CoinModelPackage.HGMutationResult, error) {
	// 钱包锁已串行化同一用户的退款；这里再读取原 debit 与退款聚合，保证累计退款不超过原扣款。
	var debitID, debitAmount, refunded uint64
	if err := tx.QueryRowContext(ctx, SQLQueriesPackage.SelectCoinDebitForRefundSQL, command.ReferenceTransactionID, command.UserID).Scan(&debitID, &debitAmount, &refunded); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CoinModelPackage.HGMutationResult{}, ErrHGRefundExceedsDebit
		}
		return CoinModelPackage.HGMutationResult{}, fmt.Errorf("select debit for refund: %w", err)
	}
	if refunded > debitAmount || command.Amount > debitAmount-refunded {
		return CoinModelPackage.HGMutationResult{}, ErrHGRefundExceedsDebit
	}
	return r.hgCredit(ctx, tx, command, balance)
}

func (r *HGRepository) hgRecordMutation(ctx context.Context, tx *sql.Tx, command CoinModelPackage.HGCommand, balanceAfter uint64) (CoinModelPackage.HGMutationResult, error) {
	// transaction 只追加不更新；request 表保存处理状态和结果，二者共同提供审计链与幂等重放。
	signedDelta := int64(command.Amount)
	if command.Operation == CoinModelPackage.HGOperationDebit || command.Operation == CoinModelPackage.HGOperationExpire {
		signedDelta = -signedDelta
	}
	if command.Operation == CoinModelPackage.HGOperationCorrection {
		signedDelta = command.SignedDelta
	}
	if command.Operation == CoinModelPackage.HGOperationInitialize {
		signedDelta = 0
	}
	result, err := tx.ExecContext(ctx, SQLQueriesPackage.InsertCoinTransactionSQL, command.UserID, command.RequestID, command.Operation, command.Amount, signedDelta, balanceAfter, command.Reason, command.BusinessType, command.BusinessKey, command.ReferenceTransactionID)
	if err != nil {
		return CoinModelPackage.HGMutationResult{}, fmt.Errorf("insert coin transaction: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return CoinModelPackage.HGMutationResult{}, fmt.Errorf("read coin transaction id: %w", err)
	}
	if _, err := tx.ExecContext(ctx, SQLQueriesPackage.CompleteCoinRequestSQL, uint64(id), balanceAfter, command.UserID, command.RequestID); err != nil {
		return CoinModelPackage.HGMutationResult{}, fmt.Errorf("complete coin request: %w", err)
	}
	return CoinModelPackage.HGMutationResult{TransactionID: uint64(id), BalanceAfter: balanceAfter}, nil
}

func hgCommandHash(command CoinModelPackage.HGCommand) string {
	value := string(command.Operation) + "\x00" + command.UserID + "\x00" + strconv.FormatUint(command.Amount, 10) + "\x00" + strconv.FormatInt(command.SignedDelta, 10) + "\x00" + command.Reason + "\x00" + command.BusinessType + "\x00" + command.BusinessKey + "\x00" + strconv.FormatUint(command.BusinessLimit, 10) + "\x00" + strconv.FormatUint(command.ReferenceTransactionID, 10)
	if command.ExpiresAt != nil {
		value += "\x00" + command.ExpiresAt.UTC().Format(time.RFC3339Nano)
	}
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
