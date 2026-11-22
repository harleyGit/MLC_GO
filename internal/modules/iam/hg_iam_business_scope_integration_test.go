package iam

import (
	ConfigPackage "MLC_GO/internal/pkg/config"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"net"
	"os"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

// 只读检查本机 debug 上新作用域授权和候选 SQL 的实际 MySQL 语法、关联列与迁移表。
// 需显式设置 MLC_BUSINESS_SCOPE_READONLY=1；不创建实体、不写角色、不访问真实业务对象。
func TestHGBusinessScopeReadonlyDebugSQL(t *testing.T) {
	if os.Getenv("MLC_BUSINESS_SCOPE_READONLY") != "1" {
		t.Skip("仅在显式指定本机 debug 只读检查时执行")
	}
	if err := ConfigPackage.LoadConfig("debug"); err != nil {
		t.Fatal(err)
	}
	cfg, err := ConfigPackage.GetMySQLConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Host != "127.0.0.1" && cfg.Host != "localhost" && cfg.Host != "::1" {
		t.Fatal("拒绝连接非本机 debug 数据库")
	}
	dsn := (&mysql.Config{User: cfg.User, Passwd: cfg.Password, Net: "tcp",
		Addr: net.JoinHostPort(cfg.Host, cfg.Port), DBName: cfg.Database}).FormatDSN()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal("无法连接本机 debug 数据库")
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var version int
	var dirty bool
	if err := db.QueryRowContext(ctx, SQLQueriesPackage.HGGetCrawlerMigrationVersionSQL).Scan(&version, &dirty); err != nil || dirty || version < 82 {
		t.Fatalf("需要至少 82/dirty=false 的迁移状态：version=%d dirty=%v err=%v", version, dirty, err)
	}
	for _, tc := range []struct {
		query string
		args  []any
	}{
		{SQLQueriesPackage.HGSelectIAMScopeResourceActive, []any{"region", "nonexistent"}},
		{SQLQueriesPackage.HGSelectIAMScopeResourcesFirst, []any{"region", 1}},
		{SQLQueriesPackage.HGSelectIAMScopeResourcesAfter, []any{"region", uint64(1), 1}},
		{SQLQueriesPackage.HGSelectBusinessOrganization, []any{"nonexistent"}},
	} {
		rows, queryErr := db.QueryContext(ctx, tc.query, tc.args...)
		if queryErr != nil {
			t.Fatal("业务作用域只读 SQL 在本机 debug 验证失败", queryErr)
		}
		for rows.Next() {
			// 只检查 SQL 实际执行和迭代是否报错，不读取或输出真实业务数据。
		}
		if queryErr = rows.Err(); queryErr != nil {
			_ = rows.Close()
			t.Fatal("业务作用域只读 SQL 在本机 debug 迭代失败", queryErr)
		}
		if queryErr = rows.Close(); queryErr != nil {
			t.Fatal("业务作用域只读 SQL 在本机 debug 关闭失败", queryErr)
		}
	}
}
