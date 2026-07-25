package HGMigratePackage

import (
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
)

func TestChekckMigrateVersionReportsMissingMigrationTable(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("创建 sqlmock 失败: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT version, dirty").
		WillReturnError(&mysql.MySQLError{Number: 1146, Message: "Table 'mlc.schema_migrations' doesn't exist"})

	err = ChekckMigrateVersion(db, 9)
	if err == nil {
		t.Fatal("schema_migrations 不存在时应返回迁移未初始化错误")
	}
	if !strings.Contains(err.Error(), "迁移记录表 schema_migrations 不存在") {
		t.Fatalf("错误提示不明确: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL 预期未满足: %v", err)
	}
}
