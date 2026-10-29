package OpsRepositoryPackage

import (
	"os"
	"strings"
	"testing"
)

func TestHGSuperAdminBootstrapMigrationsAreIdempotent(t *testing.T) {
	const hgAdminPhone = "17133853768"
	const hgPreviousAdminPhone = "17681317668"

	userMigration, err := os.ReadFile("../../../../migrations/000002_crate_user.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	userSQL := string(userMigration)
	for _, fragment := range []string{hgAdminPhone, "WHERE NOT EXISTS", "password_hash", "salt"} {
		if !strings.Contains(userSQL, fragment) {
			t.Fatalf("000002 migration missing %q", fragment)
		}
	}

	adminMigration, err := os.ReadFile("../../../../migrations/000019_bootstrap_super_admin.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	adminSQL := string(adminMigration)
	for _, fragment := range []string{
		hgAdminPhone,
		"super-admin",
		"INSERT INTO `admin_user`",
		"INSERT INTO `user_security`",
		"INSERT IGNORE INTO `admin_user_role`",
		"INSERT INTO `role_permission`",
		"NOT EXISTS",
		"INSERT INTO `user_role_view`",
	} {
		if !strings.Contains(adminSQL, fragment) {
			t.Fatalf("000019 migration missing %q", fragment)
		}
	}
	adminRollback, err := os.ReadFile("../../../../migrations/000019_bootstrap_super_admin.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(adminRollback), hgAdminPhone) {
		t.Fatal("000019 rollback missing current administrator phone")
	}
	for name, sql := range map[string]string{
		"000002": userSQL,
		"000019 up": adminSQL,
		"000019 down": string(adminRollback),
	} {
		if strings.Contains(sql, hgPreviousAdminPhone) {
			t.Errorf("%s migration still references previous administrator phone", name)
		}
	}
}
