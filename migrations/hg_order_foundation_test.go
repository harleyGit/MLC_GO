package migrations

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestHGOrderFoundationDDL 仅保护迁移文本契约，不执行 MySQL，也不证明 CHECK 或查询计划实际生效。
func TestHGOrderFoundationDDL(t *testing.T) {
	hgContent, hgErr := os.ReadFile("000037_order_foundation.up.sql")
	if hgErr != nil {
		t.Fatal(hgErr)
	}
	hgTables := make(map[string]string)
	hgPattern := regexp.MustCompile("(?s)CREATE TABLE `([^`]+)` \\(\\n(.*?)\\n\\) ENGINE=InnoDB[^;]*;")
	for _, hgMatch := range hgPattern.FindAllStringSubmatch(string(hgContent), -1) {
		if _, hgExists := hgTables[hgMatch[1]]; hgExists {
			t.Fatalf("重复建表：%s", hgMatch[1])
		}
		hgTables[hgMatch[1]] = hgMatch[2]
	}
	if len(hgTables) != 7 {
		t.Fatalf("应仅创建7张订单基础表，实际识别到%d张", len(hgTables))
	}
	hgComment := regexp.MustCompile(`COMMENT\s+'[^']*\p{Han}[^']*'`)
	for _, hgTable := range []string{"product", "product_sku", "trade_order", "trade_order_item", "payment_recharge_order", "payment_order", "refund_order"} {
		hgDDL, hgExists := hgTables[hgTable]
		if !hgExists {
			t.Fatalf("缺少基础表：%s", hgTable)
		}
		for _, hgLine := range strings.Split(hgDDL, "\n") {
			if strings.HasPrefix(strings.TrimSpace(hgLine), "`") && !hgComment.MatchString(hgLine) {
				t.Errorf("%s 字段缺少中文 COMMENT：%s", hgTable, hgLine)
			}
		}
		if !strings.Contains(hgDDL, "PRIMARY KEY (`id`)") || !strings.Contains(hgDDL, "`id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT") {
			t.Errorf("%s 缺少自增主键", hgTable)
		}
		if hgTable == "product" || hgTable == "product_sku" {
			continue
		}
		if !strings.Contains(hgDDL, "KEY `idx_"+hgTable+"_user_created_id` (`user_id`, `created_at` DESC, `id` DESC)") {
			t.Errorf("%s 缺少同向倒序用户游标索引", hgTable)
		}
	}
	for hgTable, hgRequired := range map[string][]string{
		"trade_order": {
			"UNIQUE KEY `uk_trade_order_id` (`trade_order_id`)",
			"KEY `idx_trade_order_status_expire_id` (`trade_status`, `expire_at`, `id`)",
		},
		"trade_order_item": {
			"UNIQUE KEY `uk_trade_order_item_line` (`trade_order_id`, `line_no`)",
			"CHECK (`line_no` BETWEEN 1 AND 100)",
		},
		"payment_recharge_order": {
			"`sku_id` VARCHAR(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL",
			"CHECK (`currency` = 'CNY')",
			"UNIQUE KEY `uk_payment_recharge_order_trade_id` (`trade_order_id`)",
			"KEY `idx_payment_recharge_order_status_expire_id` (`order_status`, `expire_at`, `id`)",
		},
		"payment_order": {
			"UNIQUE KEY `uk_payment_order_external` (`merchant_id`, `channel`, `external_payment_id`)",
			"KEY `idx_payment_order_status_expire_id` (`payment_status`, `expire_at`, `id`)",
		},
		"refund_order": {
			"KEY `idx_refund_order_status_expire_id` (`refund_status`, `expire_at`, `id`)",
		},
	} {
		for _, hgFragment := range hgRequired {
			if !strings.Contains(hgTables[hgTable], hgFragment) {
				t.Errorf("%s 缺少必要约束：%s", hgTable, hgFragment)
			}
		}
		if hgTable != "trade_order_item" && (!strings.Contains(hgTables[hgTable], "(`user_id`, `request_id`)") || !strings.Contains(hgTables[hgTable], "`request_hash` CHAR(64)")) {
			t.Errorf("%s 缺少用户请求幂等键或请求哈希", hgTable)
		}
	}
}

func TestHGOrderFoundationDownPreservesFacts(t *testing.T) {
	hgContent, hgErr := os.ReadFile("000037_order_foundation.down.sql")
	if hgErr != nil {
		t.Fatal(hgErr)
	}
	// 当前 down 必须只有空行或普通行注释；拒绝所有 SQL，比仅匹配 DROP/DELETE 更严格。
	for hgIndex, hgLine := range strings.Split(string(hgContent), "\n") {
		hgLine = strings.TrimSpace(hgLine)
		if hgLine != "" && !strings.HasPrefix(hgLine, "-- ") {
			t.Errorf("down 第%d行包含可执行内容，必须保留交易事实：%s", hgIndex+1, hgLine)
		}
	}
}
