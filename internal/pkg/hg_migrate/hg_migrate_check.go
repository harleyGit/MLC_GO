/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-17 21:40:09
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-07-25 19:48:52
 * @FilePath: /MLC_GO/internal/database/hg_migrate_check.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package HGMigratePackage

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/go-sql-driver/mysql"
)

// ChekckMigrateVersion 只读检测当前 migrate 版本，不负责创建或更新迁移表。
func ChekckMigrateVersion(db *sql.DB, expectVersion int) error {
	var version int
	var dirty bool

	err := db.QueryRow(`
		SELECT version, dirty
		FROM schema_migrations
		LIMIT 1
	`).Scan(&version, &dirty)

	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1146 {
			return fmt.Errorf("迁移记录表 schema_migrations 不存在，请先执行数据库迁移: %w", err)
		}
		return fmt.Errorf("无法读取迁移版本：%w", err)
	}

	if dirty {
		return fmt.Errorf("数据库处在脏的迁移状态")
	}

	if version < expectVersion {
		return fmt.Errorf(
			"数据库版本太老旧：current=%d expect >= %d",
			version, expectVersion,
		)
	}
	return nil
}
