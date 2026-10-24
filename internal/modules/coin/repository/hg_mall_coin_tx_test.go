package CoinRepositoryPackage

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestHGMarketTxDebitsAboveOldLimitWithinBoundedLotsWithoutCommitting(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.EnsureCoinWalletSQL)).WithArgs("buyer").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectCoinWalletForUpdateSQL)).WithArgs("buyer").WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(2000))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertCoinRequestSQL)).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectCoinLotsForDebitSQL)).WithArgs("buyer", now).WillReturnRows(sqlmock.NewRows([]string{"id", "remaining_amount", "expires_at"}).AddRow(1, 2000, nil))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.DebitCoinWalletSQL)).WithArgs(uint64(1500), "buyer", uint64(1500)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertCoinTransactionSQL)).WillReturnResult(sqlmock.NewResult(19, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.CompleteCoinRequestSQL)).WithArgs(uint64(19), uint64(500), "buyer", "mall:debit:MORD_1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.UpdateCoinLotRemainingSQL)).WithArgs(uint64(500), uint64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertCoinAllocationSQL)).WithArgs(uint64(19), uint64(1), uint64(1500), "debit").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertOutboxEventSQL)).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectRollback()
	repo := NewHGRepository(db, "mlc.domain.events")
	repo.hgNow = func() time.Time { return now }
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := repo.HGMarketTx(context.Background(), tx, "debit", "buyer", "MORD_1", 1500, 0)
	if err != nil || result.TransactionID != 19 || result.BalanceAfter != 500 || result.Committed {
		t.Fatalf("事务内扣款结果=%+v 错误=%v", result, err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
