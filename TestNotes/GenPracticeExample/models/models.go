/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-02-24 18:00:35
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-06 21:54:11
 * @FilePath: /MLC_GO/TestNotes/PracticeGenExample/models/models.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package models

import (
	"MLC_GO/TestNotes/GenPracticeExample/pkg/logging"
	"MLC_GO/TestNotes/GenPracticeExample/pkg/setting"
	"fmt"
	"os"
	"strings"
	"time"

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
	DeletedOn int `json:"deleted_on"`
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
		logging.ErrInfo("数据库连接失败：", err)
	}

	// 读取 blog.sql 文件中的 SQL 语句
	sqlBytes, err := os.ReadFile("./TestNotes/GenPracticeExample/docs/sql/blog.sql")
	if err != nil {
		logging.ErrInfo("failed to read sql file: ", err)
	}
	//废弃：不需要清理，实际上是多行执行造成无法创建数据表
	cleanSQL := cleanSQL(string(sqlBytes))
	// 3. 分步执行 SQL
	if err := executeSQL(db, cleanSQL); err != nil {
		logging.ErrInfo("数据库初始化失败:", err)
	}
	logging.DebugInfo("数据库初始化成功！")


	gorm.DefaultTableNameHandler = func(db *gorm.DB, defaultTableName string) string {
		return tablePrefix + defaultTableName
	}

	db.SingularTable(true)
	/*注册 Callbacks 
	*  将其注册进 GORM 的钩子里，但其本身自带 Create 和 Update 回调，因此调用替换即可
	*  在 GORM v1（老版本）中使用 Callback() 来替换 gorm:update_time_stamp 这个钩子（hook），用于更新 created_at 和 updated_at 时间戳。
	*  但 GORM v2（新版本）已经移除了 Callback() 这种用法。
	*/
	db.Callback().Create().Replace("gorm:update_time_stamp", updateTimeStampForCreateCallback)
	db.Callback().Update().Replace("gorm:update_time_stamp", updateTimeStampForUpdateCallback)
	db.Callback().Delete().Replace("gorm:delete", deleteCallback)
	db.LogMode(true)
	db.DB().SetMaxIdleConns(10)
	db.DB().SetMaxOpenConns(100)
}

func CloseDB() {
	defer db.Close()
}

func cleanSQL(raw string) string {
	// 清理 Navicat 特殊格式
	replacements := []string{
		"/* Navicat Premium Data Transfer*/", "",
		"-- ----------------------------", "",
		"/*!40101 SET @OLD_SQL_MODE=@@SQL_MODE */;", "",
	}
	replacer := strings.NewReplacer(replacements...)
	return replacer.Replace(raw)
}

/* 多行执行sql语句 */
func executeSQL(db *gorm.DB, sql string) error {
	statements := strings.Split(sql, ";")
	
	for i, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" || strings.HasPrefix(stmt, "/*") {
			continue
		}
		
		// 打印执行进度
		fmt.Printf("执行语句 %d:\n%s\n\n", i+1, stmt)
		// 执行 SQL 创建表
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("执行失败: %v\n语句: %s", err, stmt)
		}
	}
	return nil
}

func updateTimeStampForCreateCallback(scope *gorm.Scope) {
	if !scope.HasError() {
		nowTime := time.Now().Unix()
		// scope.FieldByName 通过 scope.Fields() 获取所有字段，判断当前是否包含所需字段
		// scope.FieldByName("createOn") 是 GORM v1 的 API，用于获取模型中的某个字段（这里是 "createOn"）。
		if createTimeField, ok := scope.FieldByName("createOn"); ok {
			// createTimeField.IsBlank 为 true，说明 createOn 字段当前 没有值，需要自动填充。
			if createTimeField.IsBlank {
				// 将 createOn 设置为当前时间
				createTimeField.Set(nowTime)
			}			
		}

		if  modifyTimeField, ok := scope.FieldByName("ModifiedOn"); ok {
			if modifyTimeField.IsBlank {
				modifyTimeField.Set(nowTime)
			}
		}
	}
}

// 用于 自动更新 ModifiedOn 字段（通常用于存储数据的修改时间）。
func updateTimeStampForUpdateCallback(scope *gorm.Scope) {
	// 根据入参获取设置了字面值的参数，例如本文中是 gorm:update_column ，它会去查找含这个字面值的字段属性
	// 用于检查是否手动指定了 update_column。
	if _, ok := scope.Get("gorm:update_column"); !ok {
		// 假设没有指定 update_column 的字段，我们默认在更新回调设置 ModifiedOn 的值
		// 作用是 在 ModifiedOn 为空时自动填充当前时间。
		scope.SetColumn("ModifiedOn", time.Now().Unix())
	}
}

func deleteCallback(scope *gorm.Scope) {
	if !scope.HasError() {
		var extraOption string
		// 检查是否手动指定了 delete_option
		if str, ok := scope.Get("gorm:delete_option"); ok {
			extraOption = fmt.Sprint(str)
		}

		// 获取我们约定的删除字段，若存在则 UPDATE 软删除，若不存在则 DELETE 硬删除
		// 检查模型 struct 里是否有 DeletedOn 字段。
		deletedOnField, hasDeletedOnField := scope.FieldByName("DeletedOn")

		/* 
		scope.Search.Unscoped：表示 是否跳过软删除。
			如果 Unscoped == true，说明调用了 Unscoped() 方法，应该真正删除数据。
			如果 Unscoped == false，说明 使用软删除，不删除行，而是更新 DeletedOn。
		hasDeletedOnField：只有当模型中 确实存在 DeletedOn 字段 时，才执行软删除。
		*/
		if !scope.Search.Unscoped && hasDeletedOnField {
			// 这部分 动态构造 SQL 语句，并执行更新操作
			scope.Raw(fmt.Sprintf(
				"UPDATE %v SET %v=%v%v%v",
				scope.QuotedTableName(), // 返回引用的表名，这个方法 GORM 会根据自身逻辑对表名进行一些处理
				scope.Quote(deletedOnField.DBName),// ② 返回 `deleted_on`（列名）
				scope.AddToVars(time.Now().Unix()),// 添加值作为 SQL 的参数，也可用于防范 SQL 注入
				addExtraSpaceIfExist(scope.CombinedConditionSql()),// ④ WHERE 条件; 返回组合好的条件 SQL，看一下方法原型很明了
				addExtraSpaceIfExist(extraOption), // ⑤ 额外选项（如果有）
			)).Exec()
		} else {
			scope.Raw(fmt.Sprintf(
				"DELETE FROM %v%v%v",
				scope.QuotedTableName(),
				addExtraSpaceIfExist(scope.CombinedConditionSql()),
				addExtraSpaceIfExist(extraOption),
			)).Exec()
		}
	}
}

// addExtraSpaceIfExist adds a separator
func addExtraSpaceIfExist(str string) string {
	if str != "" {
		return " " + str
	}
	return ""
}

