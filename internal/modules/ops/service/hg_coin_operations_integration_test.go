package OpsServicePackage

import (
	CoinRepositoryPackage "MLC_GO/internal/modules/coin/repository"
	CoinServicePackage "MLC_GO/internal/modules/coin/service"
	OpsDtoPackage "MLC_GO/internal/modules/ops/dto"
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// Run with MLC_OPS_COIN_INTEGRATION=1 and MLC_COIN_MYSQL_DSN to verify the complete authoritative ops flow against MySQL.
func TestHGRealMySQLOperationalCoinFlow(t *testing.T) {
	if os.Getenv("MLC_OPS_COIN_INTEGRATION") != "1" {
		t.Skip("set MLC_OPS_COIN_INTEGRATION=1 to run ops coin MySQL integration test")
	}
	db, err := sql.Open("mysql", os.Getenv("MLC_COIN_MYSQL_DSN"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	coinRepository := CoinRepositoryPackage.NewHGRepository(db, "mlc.domain.events")
	coinAssets := CoinServicePackage.NewHGService(coinRepository)
	service := NewHGOperationalService(HGOperationalDeps{Authorizer: hgFakeOpsAuthorizer{allowed: true}, CoinAssets: coinAssets, CoinQueries: coinRepository})
	userID := fmt.Sprintf("ops-coin-%d", time.Now().UnixNano())

	grant, err := service.GrantCoin(ctx, "admin-1", OpsDtoPackage.HGCoinGrantRequest{UserID: userID, RequestID: "grant-1", Amount: "10", Reason: "integration ticket", BusinessKey: "T-1"})
	if err != nil || grant.BalanceAfter != "10" {
		t.Fatalf("grant=%+v err=%v", grant, err)
	}
	replay, err := service.GrantCoin(ctx, "admin-1", OpsDtoPackage.HGCoinGrantRequest{UserID: userID, RequestID: "grant-1", Amount: "10", Reason: "integration ticket", BusinessKey: "T-1"})
	if err != nil || !replay.IdempotentReplay || replay.BalanceAfter != "10" {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
	debit, err := coinAssets.Debit(ctx, CoinServicePackage.HGDebitCommand{UserID: userID, RequestID: "debit-1", Amount: 3, Reason: "integration debit", BusinessType: "ops_test", BusinessKey: "T-1"})
	if err != nil {
		t.Fatalf("debit: %v", err)
	}
	refund, err := service.RefundCoin(ctx, "admin-1", OpsDtoPackage.HGCoinRefundRequest{UserID: userID, RequestID: "refund-1", Amount: "1", Reason: "integration refund", ReferenceTransactionID: fmt.Sprint(debit.TransactionID)})
	if err != nil || refund.BalanceAfter != "8" {
		t.Fatalf("refund=%+v err=%v", refund, err)
	}
	corrected, err := service.CorrectCoin(ctx, "admin-1", OpsDtoPackage.HGCoinCorrectionRequest{UserID: userID, RequestID: "correct-1", Delta: "-2", Reason: "confirmed drift"})
	if err != nil || corrected.BalanceAfter != "6" {
		t.Fatalf("correction=%+v err=%v", corrected, err)
	}
	account, err := service.GetCoinAccount(ctx, "admin-1", userID)
	if err != nil || account.Balance != "6" {
		t.Fatalf("account=%+v err=%v", account, err)
	}
	transactions, err := service.GetCoinTransactions(ctx, "admin-1", userID, "", 2)
	if err != nil || len(transactions.List) != 2 || !transactions.HasMore || transactions.NextCursor == "" {
		t.Fatalf("transactions=%+v err=%v", transactions, err)
	}
	next, err := service.GetCoinTransactions(ctx, "admin-1", userID, transactions.NextCursor, 2)
	if err != nil || len(next.List) == 0 {
		t.Fatalf("next=%+v err=%v", next, err)
	}
}
