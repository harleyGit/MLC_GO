package CoinRepositoryPackage

import (
	CoinEventsPackage "MLC_GO/internal/events/coin"
	CoinModelPackage "MLC_GO/internal/modules/coin/model"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
)

func TestHGRepositoryDebitConsumesLotsFEFOAndWritesOutboxAtomically(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.MatchExpectationsInOrder(false)
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	expiresSoon := now.Add(time.Hour)

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertCoinRequestSQL)).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.EnsureCoinWalletSQL)).WithArgs("user-1").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectCoinWalletForUpdateSQL)).WithArgs("user-1").WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(10))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectLegacyCoinCommandSQL)).WithArgs("user-1", "request-1").WillReturnRows(sqlmock.NewRows([]string{"submission_id", "quantity", "status"}))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectCoinBusinessDebitTotalSQL)).WithArgs("user-1", "video_coin", "video-1", "video_coin", "user-1", "video-1").WillReturnRows(sqlmock.NewRows([]string{"amount"}).AddRow(0))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectCoinLotsForDebitSQL)).WithArgs("user-1", now).WillReturnRows(
		sqlmock.NewRows([]string{"id", "remaining_amount", "expires_at"}).AddRow(2, 2, expiresSoon).AddRow(1, 8, nil),
	)
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.DebitCoinWalletSQL)).WithArgs(uint64(3), "user-1", uint64(3)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertCoinTransactionSQL)).WillReturnResult(sqlmock.NewResult(9, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.CompleteCoinRequestSQL)).WithArgs(uint64(9), uint64(7), "user-1", "request-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.UpdateCoinLotRemainingSQL)).WithArgs(uint64(0), uint64(2)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertCoinAllocationSQL)).WithArgs(uint64(9), uint64(2), uint64(2), "debit").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.UpdateCoinLotRemainingSQL)).WithArgs(uint64(7), uint64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertCoinAllocationSQL)).WithArgs(uint64(9), uint64(1), uint64(1), "debit").WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectExec("INSERT INTO outbox_events").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	repository := NewHGRepository(db, "mlc.domain.events")
	repository.hgNow = func() time.Time { return now }
	result, err := repository.Mutate(context.Background(), CoinModelPackage.HGCommand{
		Operation: CoinModelPackage.HGOperationDebit, UserID: "user-1", RequestID: "request-1", Amount: 3,
		BusinessType: "video_coin", BusinessKey: "video-1", BusinessLimit: 5,
		Event: CoinEventsPackage.HGAssetChangedEvent{UserID: "user-1", Operation: "debit", Amount: 3},
	})
	if err != nil || !result.Committed || result.BalanceAfter != 7 {
		t.Fatalf("Mutate() result=%+v err=%v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestHGRepositoryRefundCannotExceedOriginalDebit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.MatchExpectationsInOrder(false)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertCoinRequestSQL)).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.EnsureCoinWalletSQL)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectCoinWalletForUpdateSQL)).WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(5))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectCoinDebitForRefundSQL)).WillReturnRows(sqlmock.NewRows([]string{"id", "amount", "refunded"}).AddRow(10, 2, 1))
	mock.ExpectRollback()

	repository := NewHGRepository(db, "")
	_, err = repository.Mutate(context.Background(), CoinModelPackage.HGCommand{
		Operation: CoinModelPackage.HGOperationRefund, UserID: "user-1", RequestID: "refund-2", Amount: 2, ReferenceTransactionID: 10,
	})
	if !errors.Is(err, ErrHGRefundExceedsDebit) {
		t.Fatalf("error = %v, want ErrHGRefundExceedsDebit", err)
	}
}

func TestHGRepositoryRejectsBalanceOverflow(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.MatchExpectationsInOrder(false)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertCoinRequestSQL)).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.EnsureCoinWalletSQL)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectCoinWalletForUpdateSQL)).WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(int64(^uint64(0) >> 1)))
	mock.ExpectRollback()

	repository := NewHGRepository(db, "")
	_, err = repository.Mutate(context.Background(), CoinModelPackage.HGCommand{Operation: CoinModelPackage.HGOperationGrant, UserID: "user-1", RequestID: "grant-1", Amount: 1})
	if !errors.Is(err, ErrHGBalanceOverflow) {
		t.Fatalf("error = %v, want ErrHGBalanceOverflow", err)
	}
}

func TestHGRepositoryDebitLazilyCreatesOpeningLotForLegacyWallet(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.MatchExpectationsInOrder(false)
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertCoinRequestSQL)).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.EnsureCoinWalletSQL)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectCoinWalletForUpdateSQL)).WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(5))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectLegacyCoinCommandSQL)).WillReturnRows(sqlmock.NewRows([]string{"submission_id", "quantity", "status"}))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectCoinBusinessDebitTotalSQL)).WillReturnRows(sqlmock.NewRows([]string{"amount"}).AddRow(0))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectCoinLotsForDebitSQL)).WillReturnRows(sqlmock.NewRows([]string{"id", "remaining_amount", "expires_at"}))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertCoinRequestSQL)).WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertCoinTransactionSQL)).WillReturnResult(sqlmock.NewResult(8, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.CompleteCoinRequestSQL)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertCoinLotSQL)).WillReturnResult(sqlmock.NewResult(3, 1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectCoinLotsForDebitSQL)).WillReturnRows(sqlmock.NewRows([]string{"id", "remaining_amount", "expires_at"}).AddRow(3, 5, nil))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.DebitCoinWalletSQL)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertCoinTransactionSQL)).WillReturnResult(sqlmock.NewResult(9, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.CompleteCoinRequestSQL)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.UpdateCoinLotRemainingSQL)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertCoinAllocationSQL)).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	repository := NewHGRepository(db, "")
	repository.hgNow = func() time.Time { return now }
	result, err := repository.Mutate(context.Background(), CoinModelPackage.HGCommand{Operation: CoinModelPackage.HGOperationDebit, UserID: "legacy-user", RequestID: "debit-1", Amount: 2, BusinessType: "video_coin", BusinessKey: "video-1", BusinessLimit: 2})
	if err != nil || result.BalanceAfter != 3 {
		t.Fatalf("Mutate() result=%+v err=%v", result, err)
	}
}

func TestHGRepositoryRetriesDeadlockedMutation(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.MatchExpectationsInOrder(false)

	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.EnsureCoinWalletSQL)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectCoinWalletForUpdateSQL)).WillReturnError(&mysql.MySQLError{Number: 1213, Message: "deadlock"})
	mock.ExpectRollback()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertCoinRequestSQL)).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectCoinWalletForUpdateSQL)).WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(0))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.CreditCoinWalletSQL)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertCoinTransactionSQL)).WillReturnResult(sqlmock.NewResult(9, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.CompleteCoinRequestSQL)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertCoinLotSQL)).WillReturnResult(sqlmock.NewResult(3, 1))
	mock.ExpectCommit()

	repository := NewHGRepository(db, "")
	result, err := repository.Mutate(context.Background(), CoinModelPackage.HGCommand{
		Operation: CoinModelPackage.HGOperationGrant, UserID: "user-1", RequestID: "grant-1", Amount: 1,
	})
	if err != nil || !result.Committed || result.BalanceAfter != 1 {
		t.Fatalf("Mutate() result=%+v err=%v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestHGRepositoryTreatsCompletedLegacyVideoCoinAsReplay(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.EnsureCoinWalletSQL)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectCoinWalletForUpdateSQL)).WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(8))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectLegacyCoinCommandSQL)).
		WithArgs("user-1", "legacy-request").
		WillReturnRows(sqlmock.NewRows([]string{"submission_id", "quantity", "status"}).AddRow("video-1", 2, "completed"))
	mock.ExpectCommit()

	repository := NewHGRepository(db, "")
	result, err := repository.Mutate(context.Background(), CoinModelPackage.HGCommand{
		Operation: CoinModelPackage.HGOperationDebit, UserID: "user-1", RequestID: "legacy-request", Amount: 2,
		BusinessType: "video_coin", BusinessKey: "video-1", BusinessLimit: 2,
	})
	if err != nil || result.Committed || result.BalanceAfter != 8 {
		t.Fatalf("Mutate() result=%+v err=%v", result, err)
	}
}
