package gorm_practice_v

import (
	"MLC_GO/TestNotes/unfamiliar_grammar_practice/libraries/gorm_practice/gorm_practice_models"
	"MLC_GO/pkg/hglog"
	"fmt"
	"os"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// gorm数据库的连接
func GormPracticeV1_connect() {
	var (
		user string = "root"
		password string = "hh109"
		host string = "127.0.0.1:3306"
		databaseName string = "db_test"
	)


	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", user, password, host, databaseName) 
	// db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{}) 或者如下:
	db, err := gorm.Open(mysql.New(mysql.Config{
		DSN: dsn,
		DefaultStringSize: 256,  // string 类型字段的默认长度
		DisableDatetimePrecision: true, // 禁用 datetime 精度，MySQL 5.6 之前的数据库不支持
		DontSupportRenameIndex: true, // 重命名索引时采用删除并新建的方式，MySQL 5.7 之前的数据库和 MariaDB 不支持重命名索引
		DontSupportRenameColumn: true, // 用 `change` 重命名列，MySQL 8 之前的数据库和 MariaDB 不支持重命名列
		SkipInitializeWithVersion: false, // 根据当前 MySQL 版本自动配置
	}), &gorm.Config{
		Logger:                                   logger.Default.LogMode(logger.Info),
		DisableForeignKeyConstraintWhenMigrating: true,
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true, // 使用单数表名，启用该选项后，`User` 表将是`user`
		},
	})

	if err != nil {
		hglog.DebugInfo("数据库连接失败!!")
		os.Exit(0)
	}

	gormDBTables(db)
	// 从 GORM 的 *gorm.DB 对象中获取底层的标准库 *sql.DB 对象，以便直接操作连接池
	sqlDB,_ := db.DB()
	// 设置连接池中保持的 最大空闲连接数。
	// 	空闲连接可快速复用，避免每次请求新建连接。
	// 	默认值通常为 2，适当调高可提升高频小请求的性能。
	// 	设置过高会导致数据库资源浪费。
	sqlDB.SetMaxIdleConns(10)
	// 设置连接池的 最大打开连接数（同时活跃连接数上限）。
	// 	防止并发过高导致数据库连接耗尽（如 "too many connections" 错误）。
	// 	需根据数据库实际配置调整（如 MySQL 的 max_connections 参数）。
	// 	默认值为 0（无限制），生产环境必须设置合理值。
	sqlDB.SetMaxOpenConns(10)

}
// 注册数据库表GormUser专用
func gormDBTables(db *gorm.DB) {
	// 检查数据库中是否存在指定的表，若不存在则创建，并自动同步结构体与表的字段（仅添加缺失字段，不会删除/修改现有字段）。
	err := db.AutoMigrate(
		&gorm_practice_models.GormUser{},
		// &Product{},
        // &Order{}, // 你的模型结构体
	)
	if err != nil {
		hglog.ErrInfo("gorm 数据表 register table failed")
		os.Exit(0)
	}
	hglog.DebugInfo("register table success")
}