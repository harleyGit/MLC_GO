package repository

import (
	"MLC_GO/internal/modules/wallet/model"
	hgqueries "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"errors"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/spf13/viper"
)

func hgEnablePayment(t *testing.T) {
	t.Helper()
	t.Setenv("SERVER_ENV", "debug")
	t.Setenv("MLC_WALLET_DEBUG_PAYMENT_ENABLED", "true")
	hgPrevious := viper.Get("runtime.loaded_env")
	viper.Set("runtime.loaded_env", "debug")
	t.Cleanup(func() { viper.Set("runtime.loaded_env", hgPrevious) })
}

func hgPaymentRows(hgMode, hgStatus string, hgExpires time.Time, hgAmount uint64) *sqlmock.Rows {
	hgRows := sqlmock.NewRows([]string{"order_id", "user_id", "request_id", "sku_id", "display_name", "title", "description", "currency", "pay_amount", "total_coin", "created_at", "expires_at", "payment_mode", "status", "paid_at", "paid_transaction_id", "balance_after"})
	if hgStatus == "paid" {
		return hgRows.AddRow("order", "owner", "request", "sku", "name", "title", "description", "CNY", 600, hgAmount, hgExpires.Add(-10*time.Minute), hgExpires, hgMode, hgStatus, hgExpires.Add(-time.Minute), 9, 70)
	}
	return hgRows.AddRow("order", "owner", "request", "sku", "name", "title", "description", "CNY", 600, hgAmount, hgExpires.Add(-10*time.Minute), hgExpires, hgMode, hgStatus, nil, nil, nil)
}

// SQL mocks exercise the real coin repository in the wallet transaction, not a fake credit adapter.
func TestHGDebugPaymentAtomicCreditAndRollback(t *testing.T) {
	hgEnablePayment(t)
	for _, hgFailure := range []string{"", "credit", "transaction", "request", "lot", "outbox", "order", "commit"} {
		t.Run(hgFailure, func(t *testing.T) {
			hgDB, hgMock, hgErr := sqlmock.New()
			if hgErr != nil {
				t.Fatal(hgErr)
			}
			defer hgDB.Close()
			hgMock.ExpectBegin()
			hgMock.ExpectQuery(regexp.QuoteMeta(hgqueries.HGWalletOrderForPaySQL)).WithArgs("order", "owner").WillReturnRows(hgPaymentRows("platform_debug", "pending", time.Now().Add(time.Minute), 50))
			hgMock.ExpectExec(regexp.QuoteMeta(hgqueries.EnsureCoinWalletSQL)).WithArgs("owner").WillReturnResult(sqlmock.NewResult(0, 1))
			hgMock.ExpectQuery(regexp.QuoteMeta(hgqueries.SelectCoinWalletForUpdateSQL)).WithArgs("owner").WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(20))
			hgMock.ExpectExec(regexp.QuoteMeta(hgqueries.InsertCoinRequestSQL)).WithArgs("owner", "wallet_debug:order", "recharge", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
			hgInjected := errors.New("injected failure")
			for _, hgStep := range []struct{ hgName, hgSQL string }{
				{"credit", hgqueries.CreditCoinWalletSQL},
				{"transaction", hgqueries.InsertCoinTransactionSQL},
				{"request", hgqueries.CompleteCoinRequestSQL},
				{"lot", hgqueries.InsertCoinLotSQL},
				{"outbox", hgqueries.InsertOutboxEventSQL},
				{"order", hgqueries.HGWalletMarkPaidSQL},
			} {
				hgExec := hgMock.ExpectExec(regexp.QuoteMeta(hgStep.hgSQL))
				if hgStep.hgName == hgFailure {
					hgExec.WillReturnError(hgInjected)
					break
				}
				hgExec.WillReturnResult(sqlmock.NewResult(9, 1))
			}
			if hgFailure == "" {
				hgMock.ExpectCommit()
			} else if hgFailure == "commit" {
				hgMock.ExpectCommit().WillReturnError(hgInjected)
			} else {
				hgMock.ExpectRollback()
			}
			hgOrder, hgErr := HGNewRepository(hgDB).HGPayDebug(context.Background(), "owner", "order")
			if hgFailure == "" {
				if hgErr != nil || hgOrder.HGStatus != "paid" || hgOrder.HGBalanceAfter != 70 || hgOrder.HGPaymentTransactionID != 9 || hgOrder.HGPaidAt == nil {
					t.Fatalf("order=%+v err=%v", hgOrder, hgErr)
				}
			} else if !errors.Is(hgErr, hgInjected) {
				t.Fatalf("expected rollback failure: %v", hgErr)
			}
			if hgErr := hgMock.ExpectationsWereMet(); hgErr != nil {
				t.Fatal(hgErr)
			}
		})
	}
}

func TestHGDebugPaymentGuardsAndPaidReplay(t *testing.T) {
	hgEnablePayment(t)
	for _, hgCase := range []struct {
		hgName, hgEnv, hgFlag, hgMode, hgStatus string
		hgExpired                               bool
		hgAmount                                uint64
		hgWant                                  error
	}{
		{"owner", "debug", "true", "platform_debug", "pending", false, 50, model.HGErrNotFound},
		{"pre", "pre", "true", "platform_debug", "pending", false, 50, model.HGErrPaymentUnavailable},
		{"prod", "prod", "true", "platform_debug", "pending", false, 50, model.HGErrPaymentUnavailable},
		{"closed", "debug", "false", "platform_debug", "pending", false, 50, model.HGErrPaymentUnavailable},
		{"mode", "debug", "true", "unavailable", "pending", false, 50, model.HGErrPaymentUnavailable},
		{"expired", "debug", "true", "platform_debug", "pending", true, 50, model.HGErrOrderExpired},
		{"limit", "debug", "true", "platform_debug", "pending", false, 1001, model.HGErrUnsupported},
		{"zero", "debug", "true", "platform_debug", "pending", false, 0, model.HGErrUnsupported},
		{"paid-closed-expired", "debug", "false", "platform_debug", "paid", true, 50, nil},
	} {
		t.Run(hgCase.hgName, func(t *testing.T) {
			t.Setenv("SERVER_ENV", hgCase.hgEnv)
			t.Setenv("MLC_WALLET_DEBUG_PAYMENT_ENABLED", hgCase.hgFlag)
			hgDB, hgMock, hgErr := sqlmock.New()
			if hgErr != nil {
				t.Fatal(hgErr)
			}
			defer hgDB.Close()
			hgMock.ExpectBegin()
			hgQuery := hgMock.ExpectQuery(regexp.QuoteMeta(hgqueries.HGWalletOrderForPaySQL)).WithArgs("order", "owner")
			hgExpires := time.Now().Add(time.Minute)
			if hgCase.hgExpired {
				hgExpires = time.Now().Add(-time.Minute)
			}
			if hgCase.hgName == "owner" {
				hgQuery.WillReturnError(sql.ErrNoRows)
			} else {
				hgQuery.WillReturnRows(hgPaymentRows(hgCase.hgMode, hgCase.hgStatus, hgExpires, hgCase.hgAmount))
			}
			hgMock.ExpectRollback()
			hgOrder, hgErr := HGNewRepository(hgDB).HGPayDebug(context.Background(), "owner", "order")
			if !errors.Is(hgErr, hgCase.hgWant) {
				t.Fatalf("err=%v want=%v", hgErr, hgCase.hgWant)
			}
			if hgCase.hgWant == nil && (hgOrder.HGStatus != "paid" || hgOrder.HGBalanceAfter != 70) {
				t.Fatal(hgOrder)
			}
			if hgErr := hgMock.ExpectationsWereMet(); hgErr != nil {
				t.Fatal(hgErr)
			}
		})
	}
}

// Concurrent replay uses the same committed snapshot; this tests application replay paths, not InnoDB locking.
func TestHGDebugPaymentConcurrentPaidReplay(t *testing.T) {
	hgEnablePayment(t)
	hgDB, hgMock, hgErr := sqlmock.New()
	if hgErr != nil {
		t.Fatal(hgErr)
	}
	defer hgDB.Close()
	hgMock.MatchExpectationsInOrder(false)
	const hgCount = 16
	for hgIndex := 0; hgIndex < hgCount; hgIndex++ {
		hgMock.ExpectBegin()
		hgMock.ExpectQuery(regexp.QuoteMeta(hgqueries.HGWalletOrderForPaySQL)).WithArgs("order", "owner").WillReturnRows(hgPaymentRows("platform_debug", "paid", time.Now().Add(-time.Minute), 50))
		hgMock.ExpectRollback()
	}
	hgRepo := HGNewRepository(hgDB)
	var hgWait sync.WaitGroup
	for hgIndex := 0; hgIndex < hgCount; hgIndex++ {
		hgWait.Add(1)
		go func() {
			defer hgWait.Done()
			hgOrder, hgErr := hgRepo.HGPayDebug(context.Background(), "owner", "order")
			if hgErr != nil || hgOrder.HGBalanceAfter != 70 || hgOrder.HGPaymentTransactionID != 9 {
				t.Errorf("order=%+v err=%v", hgOrder, hgErr)
			}
		}()
	}
	hgWait.Wait()
	if hgErr := hgMock.ExpectationsWereMet(); hgErr != nil {
		t.Fatal(hgErr)
	}
}
