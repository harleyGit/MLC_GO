package permission

import (
	"os"
	"regexp"
	"testing"
)

// 对照已版本化的 IAM 种子 SQL，防止枚举拼写漂移、漏登记或误删；不连接数据库。
func TestHGPermissionEnumMatchesMigrations(t *testing.T) {
	hgExpected := make(map[string]bool)
	hgPattern := regexp.MustCompile(`'([a-z][a-z0-9_]*:[a-z][a-z0-9_]*:[a-z][a-z0-9_]*)'`)
	for _, hgFile := range []string{"000046_create_iam.up.sql", "000047_iam_cutover_security.up.sql", "000048_route_permissions.up.sql"} {
		hgSQL, hgErr := os.ReadFile("../../../migrations/" + hgFile)
		if hgErr != nil {
			t.Fatal(hgErr)
		}
		for _, hgMatch := range hgPattern.FindAllStringSubmatch(string(hgSQL), -1) {
			hgExpected[hgMatch[1]] = true
		}
	}
	hgSeen := make(map[HGPermission]bool)
	for _, hgPermission := range HGPermissions() {
		if hgSeen[hgPermission] || !hgPermission.HGKnown() || !hgExpected[string(hgPermission)] {
			t.Fatalf("权限枚举重复、未登记或与迁移不一致: %s", hgPermission)
		}
		hgSeen[hgPermission] = true
		delete(hgExpected, string(hgPermission))
	}
	if len(hgExpected) > 0 {
		t.Fatalf("迁移权限缺少枚举: %v", hgExpected)
	}
	if HGPermission("").HGKnown() || HGPermission("unknown:resource:read").HGKnown() {
		t.Fatal("未知权限被当作有效枚举")
	}
	hgCopy := HGPermissions()
	hgCopy[0] = "changed"
	if !HGIAMRoleRead.HGKnown() || HGPermission("changed").HGKnown() {
		t.Fatal("返回字典副本改变了权限校验")
	}
}
