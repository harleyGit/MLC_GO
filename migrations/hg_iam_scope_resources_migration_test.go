package migrations

import (
	"os"
	"strings"
	"testing"
)

// 新作用域迁移只操作连接目标数据库；隔离验收不可被 USE 固定库名重定向到共享库。
func TestHGIAMScopeResourcesMigrationUsesSelectedDatabase(t *testing.T) {
	up, err := os.ReadFile("000080_iam_scope_resources.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile("000080_iam_scope_resources.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	for filename, text := range map[string]string{
		"up":   string(up),
		"down": string(down),
	} {
		for _, line := range strings.Split(text, "\n") {
			if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(line)), "USE ") {
				t.Errorf("%s 迁移不得重定向数据库: %q", filename, line)
			}
		}
	}
	sqlText := string(up)
	for _, required := range []string{
		"CREATE TABLE iam_scope_resources",
		"UNIQUE KEY uk_iam_scope_resource(scope_type,scope_id)",
		"UNIQUE KEY uk_iam_scope_resource_request(created_by,request_id)",
		"KEY idx_iam_scope_resource_page(scope_type,status,id)",
		"UPDATE iam_scope_types SET status=1 WHERE scope_code IN",
		"'creator','organization','environment','crawler_project','content_channel','region'",
	} {
		if !strings.Contains(sqlText, required) {
			t.Errorf("迁移缺少预期约束或类型: %q", required)
		}
	}
	if !strings.Contains(string(down), "SIGNAL SQLSTATE '45000'") ||
		strings.Contains(strings.ToUpper(string(down)), "DROP TABLE") {
		t.Error("下行迁移必须阻止自动删除已有资源及授权依据")
	}
}
