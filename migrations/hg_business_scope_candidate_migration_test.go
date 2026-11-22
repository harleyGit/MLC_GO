package migrations

import (
	"os"
	"strings"
	"testing"
)

// 占位 IAM 记录默认不可作为真实业务资源，候选查询需按验证位有界翻页。
func TestHGBusinessScopeCandidateMigration(t *testing.T) {
	up, err := os.ReadFile("000082_business_scope_candidate_index.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile("000082_business_scope_candidate_index.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"ALTER TABLE iam_scope_resources", "business_verified TINYINT NOT NULL DEFAULT 0",
		"idx_iam_scope_business_page(scope_type,status,business_verified,id)",
	} {
		if !strings.Contains(string(up), required) {
			t.Errorf("业务资源候选索引缺少 %s", required)
		}
	}
	for _, line := range strings.Split(string(up)+"\n"+string(down), "\n") {
		upper := strings.ToUpper(strings.TrimSpace(line))
		if strings.HasPrefix(upper, "USE ") || strings.HasPrefix(upper, "UPDATE ") ||
			strings.HasPrefix(upper, "DELETE FROM ") {
			t.Errorf("不得重定向数据库或自动提升历史占位记录: %q", line)
		}
	}
	if !strings.Contains(string(down), "SIGNAL SQLSTATE '45000'") {
		t.Fatal("业务资源验证位不允许自动回滚")
	}
}
