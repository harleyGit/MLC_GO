/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-14 20:22:42
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-17 23:26:21
 * @FilePath: /MLC_GO/internal/infrastructure/persistence/mysql/sql.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package PersistenceSQLPackage

import (
	ConfigPackage "MLC_GO/internal/config"
	SQLQueriesPackage "MLC_GO/internal/infrastructure/persistence/mysql/queries"
	UserModelsPackage "MLC_GO/internal/models/user_models"
	"MLC_GO/internal/pkg/logHG"
	"database/sql"
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

var (
	db *sql.DB
)

func LoadSQLEnvValue() {
	// 仅在本地开发时加载 .env 文件
    // 如果是生产环境，.env 文件不存在，这步会失败，但没关系
	if os.Getenv("APP_ENV") != ConfigPackage.PROD {
		// 默认尝试加载当前工作目录下的 .env 文件。
		// err := godotenv.Load()
		err := godotenv.Load(ConfigPackage.DEV_ENV_PATH)
		if err != nil {
			logHG.ErrInfo("警告： No .env 文件没有找到")
		}
	}
	wd, _ := os.Getwd()
	logHG.DebugInfo("加载sql环境变量,当前工作目录：%v", wd)
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return  value
	}
	return  fallback
}

// 分环境加载环境变量
func getSQLDSNV2() string {
	cfg := ConfigPackage.Load()
	var sqlDSN = SQLQueriesPackage.DB_DSN
	logHG.DebugFInfo("++++ cfg.MAC_TYPE: %v",cfg.MAC_TYPE)

	if cfg.MAC_TYPE == ConfigPackage.DEV_COMPUTER {
		sqlDSN = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=UTC", 
		cfg.MySQLUser, cfg.MySQLPass, cfg.MySQLHost, cfg.MySQLPort, cfg.MySQLDB)
		fmt.Printf("🍎 Raw DSN: %s\n", sqlDSN)
	}
	logHG.DebugFInfo("SQLDSN: %v",sqlDSN)

	return  sqlDSN
}
// Deprecated: 使用 getSQLDSNV2 方法，因为这个方法需要使用默认 .env 文件内容
func getSQLDSN() string {
	host := getEnv("MYSQL_HOST", "localhost")
	port := getEnv("MYSQL_PORT", "3306")
	user := getEnv("MYSQL_USER", "root")
	password := getEnv("MYSQL_PASSWORD", "hh109")
	dbName := getEnv("MYSQL_DB", "HG_MLC_DB")
	macType := getEnv("MAC_TYPE", "M2Pro")
	var sqlDSN = SQLQueriesPackage.DB_DSN

	if macType == ConfigPackage.DEV_COMPUTER {
		sqlDSN = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=UTC", 
		user, password, host, port, dbName)
	}
	logHG.DebugFInfo("SQLDSN: %v",sqlDSN)
	
	return  sqlDSN
}

func NewSQLDB()(*sql.DB, error) {
	var err error
	// dsn := getSQLDSN()//SQLQueriesPackage.DB_DSN
	dsn := getSQLDSNV2()
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		logHG.ErrFInfo("连接MySQL数据库失败: %v", err)
		return nil, err
	}
	if err = db.Ping(); err != nil {
		logHG.ErrFInfo("Ping MySQL数据库失败: %v", err)
		return nil, err
	}
	// Go 程序启动时校验数据库（不建表）
	if _, err := checkoutSQLTable();  err != nil {
		return  nil, err
	}

	return db, nil
}

/* 启动即校验 schema,建立表时进行校验 */
func checkoutSQLTable() (*sql.DB, error) {
	// 启动即校验 schema
	if _, err := db.Exec("SELECT 1 FROM users LIMIT 1"); err != nil {
		// 表不存在 → 程序直接失败
		// 这是“部署错误”，不是“运行时错误”
		return nil, fmt.Errorf("database schema not ready: %w", err)
	}
	
	return  db, nil
}



/* 创建用户 */
func CreateUser(u *UserModelsPackage.HGUserModel) error {
	stmt, err := db.Exec(SQLQueriesPackage.InsertUserSQL, 
		u.Email, u.Phone, u.PasswordHash, u.Salt)
	if err != nil {
		return err
	}
	_, err = stmt.LastInsertId()
	return err
}
/* 获取用户信息 */
func GetUserByEmail(account string) (*UserModelsPackage.HGUserModel, error) {
	row := db.QueryRow(SQLQueriesPackage.GetUserByEmailOrPhoneSQL, account, account)

	u := &UserModelsPackage.HGUserModel{}
	err := row.Scan(&u.UserID, &u.Email, &u.Phone, &u.PasswordHash, &u.Salt)
	if err != nil {
		return nil, err
	}
	return u, nil
}