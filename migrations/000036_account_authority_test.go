package migrations_test

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestHGAccountAuthorityMigrationPreservesFinancialShape(t *testing.T) {
	hgUp, hgErr := os.ReadFile("000036_account_authority.up.sql")
	if hgErr != nil {
		t.Fatal(hgErr)
	}
	hgSQL := string(hgUp)
	hgExecutable := regexp.MustCompile(`(?m)--[^\n]*`).ReplaceAllString(hgSQL, "")
	for _, hgFragment := range []string{
		"RENAME TABLE `user_coin_wallets` TO `account`",
		"`coin_asset_transactions` TO `account_ledger`",
		"ADD COLUMN `frozen_balance`",
		"ADD COLUMN `currency` VARCHAR(16) NOT NULL DEFAULT 'MLC_COIN'",
		"ADD COLUMN `account_type`",
		"ADD COLUMN `version`",
		"GENERATED ALWAYS AS (",
		"CAST(`balance_after` AS DECIMAL(20,0)) - CAST(`signed_delta` AS DECIMAL(20,0))",
		"ALGORITHM=INSTANT",
		"ALGORITHM=INPLACE, LOCK=NONE",
	} {
		if !strings.Contains(hgSQL, hgFragment) {
			t.Fatalf("migration 36 缺少关键片段: %s", hgFragment)
		}
	}
	for _, hgForbidden := range []string{"INSERT", "UPDATE", "DELETE", "DROP", "TRUNCATE", "CREATE", "MODIFY", "CHANGE", "PRIMARY KEY", "ADD INDEX", "ADD KEY", "STORED"} {
		if strings.Contains(strings.ToUpper(hgExecutable), hgForbidden) {
			t.Fatalf("migration 36 不得复制、删除或清空资金数据: %s", hgForbidden)
		}
	}
	// 仅改名加列且不触碰索引，原 user_id 聚簇主键和流水索引随表保留。
	if strings.Count(hgExecutable, "ADD COLUMN") != 5 || strings.Count(hgExecutable, "COMMENT '") != 5 || !regexp.MustCompile(`(?s)ADD COLUMN `+"`balance_before`"+` DECIMAL\(20,0\).*?\) VIRTUAL`).MatchString(hgExecutable) {
		t.Fatal("新增列必须含中文 COMMENT，balance_before 必须为 VIRTUAL DECIMAL")
	}
	if strings.Count(hgExecutable, ";") != 3 {
		t.Fatal("up 仅允许原表改名和两次加列，不得另建索引或改写历史数据")
	}
}

func TestHGAccountAuthorityDownIsExplicitlyNonRoundTrip(t *testing.T) {
	hgDown, hgErr := os.ReadFile("000036_account_authority.down.sql")
	if hgErr != nil {
		t.Fatal(hgErr)
	}
	hgSQL := string(hgDown)
	hgExecutable := regexp.MustCompile(`(?m)--[^\n]*`).ReplaceAllString(hgSQL, "")
	if strings.Join(strings.Fields(hgExecutable), " ") != "RENAME TABLE `account` TO `user_coin_wallets`, `account_ledger` TO `coin_asset_transactions`;" {
		t.Fatal("down 仅允许恢复表名，不得修改资金列或索引")
	}
	for _, hgFragment := range []string{
		"RENAME TABLE `account` TO `user_coin_wallets`",
		"`account_ledger` TO `coin_asset_transactions`",
		"不删除新增列",
		"不是可直接 down/up 的可逆迁移",
		"禁止降级旧程序",
	} {
		if !strings.Contains(hgSQL, hgFragment) {
			t.Fatalf("down migration 未明确资金兼容边界: %s", hgFragment)
		}
	}
	for _, hgForbidden := range []string{"DROP TABLE", "DROP COLUMN", "DELETE FROM", "TRUNCATE"} {
		if strings.Contains(strings.ToUpper(hgSQL), hgForbidden) {
			t.Fatalf("down migration 不得静默删除资金数据: %s", hgForbidden)
		}
	}
}
