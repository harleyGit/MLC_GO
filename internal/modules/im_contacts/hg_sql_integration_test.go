package im_contacts

import (
	ConfigPackage "MLC_GO/internal/pkg/config"
	MySQLPackage "MLC_GO/internal/pkg/mysql"
	SQL "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

// TestHGContactListSQLAgainstDebug 只读检查三个列表在 debug 库的真实表结构及 JOIN 排序规则。
// 显式启用后仅使用空用户 ID 做只读查询，不输出资料、不运行迁移；默认单元测试不会连接数据库。
func TestHGContactListSQLAgainstDebug(t *testing.T) {
	if os.Getenv("MLC_IM_CONTACTS_SQL_INTEGRATION") != "1" {
		t.Skip("设置 MLC_IM_CONTACTS_SQL_INTEGRATION=1 才检查 debug MySQL")
	}
	if os.Getenv("SERVER_ENV") != "debug" {
		t.Fatal("只允许在 SERVER_ENV=debug 下运行联系人只读 SQL 检查")
	}
	t.Setenv("MLC_CONFIG_DIR", "../../../config")
	if err := ConfigPackage.LoadConfig("debug"); err != nil {
		t.Fatal("加载 debug 配置失败")
	}
	cfg, err := ConfigPackage.GetMySQLConfig()
	if err != nil {
		t.Fatal("读取 debug MySQL 配置失败")
	}
	if cfg.Host != "127.0.0.1" && cfg.Host != "localhost" && cfg.Host != "::1" {
		t.Fatal("联系人只读 SQL 检查仅允许连接本地 debug MySQL")
	}
	db, err := MySQLPackage.NewSQLDB()
	if err != nil {
		t.Fatal("连接 debug MySQL 失败")
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	service := HGNewService(db)
	for _, check := range []struct {
		name string
		run  func() error
	}{
		{"friends", func() error { _, err := service.HGList(ctx, "", 0); return err }},
		{"incoming", func() error { _, err := service.HGIncoming(ctx, "", 0); return err }},
		{"blocks", func() error { _, err := service.HGBlocks(ctx, "", 0); return err }},
	} {
		if err := check.run(); err != nil {
			var mysqlErr *mysql.MySQLError
			if errors.As(err, &mysqlErr) {
				t.Errorf("%s 列表 SQL 在 debug MySQL 执行失败，MySQL 错误码: %d", check.name, mysqlErr.Number)
			} else {
				t.Errorf("%s 列表 SQL 在 debug MySQL 执行失败: %T", check.name, err)
			}
		}
	}
	// 用不会指向真实账号的值验证新增的索引点查 SQL 可执行，不遍历或输出用户资料。
	for _, check := range []struct{ name, query string }{
		{"phone", SQL.HGIMContactSearchPhoneSQL},
		{"email", SQL.HGIMContactSearchEmailSQL},
	} {
		rows, err := db.QueryContext(ctx, check.query, "")
		if err != nil {
			var mysqlErr *mysql.MySQLError
			if errors.As(err, &mysqlErr) {
				t.Errorf("%s 搜索 SQL 在 debug MySQL 执行失败，MySQL 错误码: %d", check.name, mysqlErr.Number)
			} else {
				t.Errorf("%s 搜索 SQL 在 debug MySQL 执行失败: %T", check.name, err)
			}
			continue
		}
		if err := rows.Close(); err != nil {
			t.Errorf("%s 搜索 SQL 关闭结果失败: %T", check.name, err)
		}
	}
}
