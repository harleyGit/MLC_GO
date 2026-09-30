package CoinRepositoryPackage

import (
	hgqueries "MLC_GO/internal/pkg/mysql/queries"
	"strings"
	"testing"
)

// 不以 SQL 常量自身作为期望，独立约束权威表名、绑定参数及余额版本更新。
func TestHGAccountAuthoritySQLContract(t *testing.T) {
	for hgName, hgCase := range map[string]struct {
		hgSQL   string // hgSQL 是待验证的集中 SQL 常量。
		hgTable string // hgTable 是迁移后的权威表名。
		hgArgs  int    // hgArgs 是必须兼容的原绑定参数数量。
	}{
		"ensure":      {hgqueries.EnsureCoinWalletSQL, "account", 1},
		"balance":     {hgqueries.SelectCoinWalletSQL, "account", 1},
		"lock":        {hgqueries.SelectCoinWalletForUpdateSQL, "account", 1},
		"credit":      {hgqueries.CreditCoinWalletSQL, "account", 3},
		"debit":       {hgqueries.DebitCoinWalletSQL, "account", 3},
		"transaction": {hgqueries.InsertCoinTransactionSQL, "account_ledger", 10},
		"refund":      {hgqueries.SelectCoinDebitForRefundSQL, "account_ledger", 2},
		"business":    {hgqueries.SelectCoinBusinessDebitTotalSQL, "account_ledger", 6},
		"first":       {hgqueries.SelectCoinTransactionsFirstSQL, "account_ledger", 2},
		"cursor":      {hgqueries.SelectCoinTransactionsByCursorSQL, "account_ledger", 5},
		"reconcile":   {hgqueries.SelectCoinReconciliationPageSQL, "account", 2},
		"consolidate": {hgqueries.SelectCoinConsolidationUsersSQL, "account", 3},
	} {
		t.Run(hgName, func(t *testing.T) {
			if !strings.Contains(hgCase.hgSQL, hgCase.hgTable) || strings.Contains(hgCase.hgSQL, "user_coin_wallets") || strings.Contains(hgCase.hgSQL, "coin_asset_transactions") || strings.Contains(hgCase.hgSQL, "user_coin_ledger") || strings.Count(hgCase.hgSQL, "?") != hgCase.hgArgs {
				t.Fatalf("权威 SQL 契约改变: %s", hgName)
			}
		})
	}
	for _, hgSQL := range []string{hgqueries.CreditCoinWalletSQL, hgqueries.DebitCoinWalletSQL} {
		if !strings.Contains(hgSQL, "version = version + 1") {
			t.Fatal("余额写入必须递增版本")
		}
	}
	if !strings.Contains(hgqueries.InsertOutboxEventSQL, "INSERT INTO outbox_event (") {
		t.Fatal("Outbox 必须使用单数表名")
	}
}
