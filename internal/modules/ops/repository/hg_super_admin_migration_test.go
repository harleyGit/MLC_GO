package OpsRepositoryPackage

import (
	"os"
	"strings"
	"testing"
)

func TestHGSuperAdminBootstrapMigrationsAreIdempotent(t *testing.T) {
	userMigration, err := os.ReadFile("../../../../migrations/000002_crate_user.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	userSQL := string(userMigration)
	for _, fragment := range []string{"17681317668", "WHERE NOT EXISTS", "password_hash", "salt"} {
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
		"17681317668",
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
}
