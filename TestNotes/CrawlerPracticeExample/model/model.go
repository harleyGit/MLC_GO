/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-05 11:23:59
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-06 18:01:49
 * @FilePath: /MLC_GO/TestNotes/CrawlerPracticeExample/model/model.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package model

import (
	// "MLC_GO/TestNotes/CrawlerPracticeExample/parse"
	"MLC_GO/TestNotes/CrawlerPracticeExample/parse"
	"MLC_GO/TestNotes/GenPracticeExample/pkg/logging"
	"fmt"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	// "github.com/jinzhu/gorm"
	// "github.com/jinzhu/gorm/dialects/mysql" //注册 MySQL 驱动，让 gorm.Open("mysql", dsn) 识别 "mysql" 这个驱动
)

var (
	DB *gorm.DB

	username string = "root"
	password string = "hh109"
	dbName string = "db_test"
)

func init() {
	var err error
	dsn := fmt.Sprintf("%s:%s@tcp(127.0.0.1:3306)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		"root", "hh109", "db_test")

	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		logging.ErrInfo("数据库连接失败 gorm.Open.err: ", err)
	}

	// gorm.DefaultTableNameHandler = func(db *gorm.DB, defaultTableName string) string {
	// 	logging.DebugInfo("数据库名字：", ("sp_" + defaultTableName))
	// 	return "sp_" + defaultTableName
	// }

	// DB.SingularTable(true)
	DB.AutoMigrate(&parse.DoubanMovie{})
}

// CloseDB 关闭数据库连接
func CloseDB() {
	sqlDB, err := DB.DB() // 获取底层 *sql.DB
	if err != nil {
		logging.ErrInfo("获取数据库连接失败:", err)
		return
	}

	err = sqlDB.Close()
	if err != nil {
		logging.ErrInfo("关闭数据库连接失败:", err)
	} else {
		logging.DebugInfo("数据库连接已关闭")
	}
}
