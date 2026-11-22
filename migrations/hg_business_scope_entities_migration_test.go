package migrations

import (
	"os"
	"strings"
	"testing"
)

// 六类业务实体必须独立于 IAM 占位数据，旧对象不能按名称或字符串自动获得区域/频道等授权。
func TestHGBusinessScopeEntitiesMigration(t *testing.T) {
	up, err := os.ReadFile("000081_business_scope_entities.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile("000081_business_scope_entities.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"hg_creator_spaces", "hg_business_organizations", "hg_organization_members",
		"hg_operation_environments", "hg_crawler_projects", "hg_content_channels",
		"hg_business_regions", "hg_crawler_task_projects", "hg_submission_scope_links",
		"hg_merchant_regions",
	} {
		if !strings.Contains(string(up), "CREATE TABLE "+name+" (") {
			t.Errorf("缺少权威实体或关联表 %s", name)
		}
	}
	for _, index := range []string{
		"idx_hg_creator_owner_status", "idx_hg_org_member_user", "uk_hg_environment_code",
		"idx_hg_project_task", "idx_hg_submission_creator", "idx_hg_submission_channel",
		"idx_hg_region_merchant", "uk_hg_region_code",
	} {
		if !strings.Contains(string(up), index) {
			t.Errorf("缺少有界查询索引 %s", index)
		}
	}
	for _, line := range strings.Split(string(up)+"\n"+string(down), "\n") {
		upper := strings.ToUpper(strings.TrimSpace(line))
		if strings.HasPrefix(upper, "USE ") || strings.HasPrefix(upper, "INSERT INTO ") ||
			strings.HasPrefix(upper, "UPDATE ") || strings.HasPrefix(upper, "DELETE FROM ") {
			t.Errorf("不得重定向数据库或盲目回填旧数据: %q", line)
		}
	}
	if !strings.Contains(string(down), "SIGNAL SQLSTATE '45000'") || strings.Contains(strings.ToUpper(string(down)), "DROP TABLE") {
		t.Error("不能自动删除业务实体及已产生的授权关联")
	}
}
