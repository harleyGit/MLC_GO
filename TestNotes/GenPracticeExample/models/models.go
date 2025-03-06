/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-02-24 18:00:35
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-06 19:43:55
 * @FilePath: /MLC_GO/TestNotes/PracticeGenExample/models/models.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package models

import (
	"MLC_GO/TestNotes/GenPracticeExample/pkg/logging"
	"MLC_GO/TestNotes/GenPracticeExample/pkg/setting"
	"fmt"
	"io/ioutil"

	_ "github.com/jinzhu/gorm/dialects/mysql" //注册 MySQL 驱动，让 gorm.Open("mysql", dsn) 识别 "mysql" 这个驱动

	"github.com/jinzhu/gorm"
)

// db 是一个 全局数据库连接对象，类型为 *gorm.DB，用于执行数据库操作
var db *gorm.DB

// 这个 Model 结构体通常用作所有数据库表的基础模型，包含常见的字段，如 主键 ID、创建时间、修改时间
type Model struct {
	// gorm:"primary_key"：指定 ID 作为主键（Primary Key）。
	// json:"id"：定义 JSON 序列化时的字段名称，JSON 输出时键名是 id（注意 json: 后面不应该有空格）
	ID int `gorm:"primary_key" json:"id"`
	CreatedOn int `json:"created_on"`
	// ModifiedOn 表示修改时间，用于记录最近的更新时间。
	ModifiedOn int `json:"modified_on"`
}

func Setup() {
	var (
		err error
		dbType, dbName, user, password, host, tablePrefix string
	)

	sec, err := setting.Cfg.GetSection("database")
	if err != nil {
		logging.Fatal(2, "Fail to get section 'database': %v", err)
	}

	dbType = sec.Key("TYPE").String()
	dbName = sec.Key("NAME").String()
	user = sec.Key("USER").String()
	password = sec.Key("PASSWORD").String()
	host = sec.Key("HOST").String()
	tablePrefix = sec.Key("TABLE_PREFIX").String()

	// 连接数据库，使用 GORM 作为 ORM（对象关系映射），并且使用 fmt.Sprintf() 生成 MySQL 连接字符串
	db, err = gorm.Open(dbType, fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8&parseTime=True&loc=Local",
		user,
		password,
		host,
		dbName))

	if err != nil {
		logging.Info(err)
	}

	// 读取 blog.sql 文件中的 SQL 语句
	sqlContent, err := ioutil.ReadFile("MLC_GO/TestNotes/GenPracticeExample/docs/sql/blog.sql")
	if err != nil {
		logging.ErrInfo("failed to read sql file: ", err)
	}
	// 执行 SQL 创建表
	sqlStr := string(sqlContent)
	err = db.Exec(sqlStr).Error
	if err != nil {
		logging.ErrInfo("failed to execute SQL: ", err)
	}


	gorm.DefaultTableNameHandler = func(db *gorm.DB, defaultTableName string) string {
		return tablePrefix + defaultTableName
	}

	db.SingularTable(true)
	db.LogMode(true)
	db.DB().SetMaxIdleConns(10)
	db.DB().SetMaxOpenConns(100)
}

func CloseDB() {
	defer db.Close()
}

