/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-14 20:22:42
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-05-30 22:26:33
 * @FilePath: /MLC_GO/internal/pkg/mysql/sql.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package PersistenceSQLPackage

import (
	UserModelsPackage "MLC_GO/internal/modules/user/model"
	ConfigPackage "MLC_GO/internal/pkg/config"
	"MLC_GO/internal/pkg/logHG"
	HGLoggerPackage "MLC_GO/internal/pkg/logger"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/go-sql-driver/mysql"
)

var (
	db *sql.DB
)

type HGSQLManager struct {
	db *sql.DB
}

const (
	// defaultSQLMaxOpenConns 控制单实例最多同时打开多少条 MySQL 连接。
	// 这个值不能无限大：应用实例数 * MaxOpenConns 不能超过数据库可承受连接数。
	defaultSQLMaxOpenConns = 100
	// defaultSQLMaxIdleConns 保留部分空闲连接，减少高峰期频繁建连成本。
	defaultSQLMaxIdleConns = 50
	// defaultSQLConnMaxLifetime 控制连接最大生命周期，避免长连接一直绑定到旧后端或被中间网络设备悄悄断开。
	defaultSQLConnMaxLifetime = 30 * time.Minute
	// defaultSQLConnMaxIdleTime 控制空闲连接最长保留时间，低峰期主动释放多余连接。
	defaultSQLConnMaxIdleTime = 5 * time.Minute
	// defaultSQLPingTimeout 限制启动期 Ping 耗时，避免数据库不可达时进程卡死。
	defaultSQLPingTimeout = 5 * time.Second
)

func buildSQLDSN(cfg ConfigPackage.HGMySQLConfig) string {
	return (&mysql.Config{
		User:      cfg.User,
		Passwd:    cfg.Password,
		Net:       "tcp",
		Addr:      cfg.Host + ":" + cfg.Port,
		DBName:    cfg.Database,
		ParseTime: true,
		Loc:       time.UTC,
		Collation: "utf8mb4_general_ci",
	}).FormatDSN()
}

func NewSQLManager() (*HGSQLManager, error) {
	// db1, err : = NewSQLDB()
	db, err := NewSQLDB()
	if err != nil {
		return nil, err
	}
	sqlManager := &HGSQLManager{db: db}
	return sqlManager, nil
}

func NewSQLDB() (*sql.DB, error) {
	var err error
	cfg, err := ConfigPackage.GetMySQLConfig()
	if err != nil {
		return nil, err
	}
	dsn := buildSQLDSN(cfg)
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		logHG.ErrFInfo("连接MySQL数据库失败: %v", err)
		return nil, err
	}
	// sql.Open 只创建连接池句柄，不会立即建立真实网络连接。
	// 因此必须先配置连接池，再 PingContext 校验 DSN、网络和账号权限。
	configureSQLPool(db)

	pingCtx, cancel := context.WithTimeout(context.Background(), defaultSQLPingTimeout)
	defer cancel()
	if err = db.PingContext(pingCtx); err != nil {
		logHG.ErrFInfo("Ping MySQL数据库失败: %v", err)
		_ = db.Close()
		return nil, err
	}
	// Go 程序启动时校验数据库（不建表）
	if _, err := checkoutSQLTable(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

func (sqlManger *HGSQLManager) GetSQLDB() *sql.DB {
	return sqlManger.db
}

// Close 释放 SQL 连接池，供服务优雅关闭时调用。
// 如果不关闭连接池，进程退出前可能残留连接和后台清理 goroutine，不利于测试和优雅发布。
func (sqlManger *HGSQLManager) Close() error {
	if sqlManger == nil || sqlManger.db == nil {
		return nil
	}
	return sqlManger.db.Close()
}

// PingContext 检查数据库连接是否可用，供 readyz 使用。
// 使用调用方传入的 context，可以让 /readyz 对依赖检查设置明确超时，避免探活请求被数据库卡住。
func (sqlManger *HGSQLManager) PingContext(ctx context.Context) error {
	if sqlManger == nil || sqlManger.db == nil {
		return sql.ErrConnDone
	}
	return sqlManger.db.PingContext(ctx)
}

// configureSQLPool 设置数据库连接池，默认值可通过环境变量覆盖以适配不同规格。
//
// 为什么连接池要可配置：
// 1. 本地开发、单机部署、K8s 多副本对连接数的需求完全不同。
// 2. 数据库总连接数是全局资源，应用实例越多，单实例连接池越要保守。
// 3. ConnMaxLifetime/ConnMaxIdleTime 可以降低坏连接、旧连接和低峰空闲连接带来的不稳定性。
func configureSQLPool(db *sql.DB) {
	maxOpen := getEnvInt("MYSQL_MAX_OPEN_CONNS", defaultSQLMaxOpenConns)
	maxIdle := getEnvInt("MYSQL_MAX_IDLE_CONNS", defaultSQLMaxIdleConns)
	if maxIdle > maxOpen {
		maxIdle = maxOpen
	}

	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(getEnvDuration("MYSQL_CONN_MAX_LIFETIME", defaultSQLConnMaxLifetime))
	db.SetConnMaxIdleTime(getEnvDuration("MYSQL_CONN_MAX_IDLE_TIME", defaultSQLConnMaxIdleTime))
}

func getEnvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err == nil && parsed > 0 {
		return parsed
	}
	seconds, err := strconv.Atoi(value)
	if err != nil || seconds <= 0 {
		return fallback
	}
	return time.Duration(seconds) * time.Second
}

/* 启动即校验 schema,建立表时进行校验 */
func checkoutSQLTable() (*sql.DB, error) {
	// 启动即校验 schema
	if _, err := db.Exec("SELECT 1 FROM users LIMIT 1"); err != nil {
		// 表不存在 → 程序直接失败
		// 这是“部署错误”，不是“运行时错误”
		return nil, fmt.Errorf("database schema not ready: %w", err)
	}

	return db, nil
}

/* 创建用户 */
func CreateUser(u *UserModelsPackage.HGUserModel) error {
	stmt, err := db.Exec(SQLQueriesPackage.InsertUserSQL,
		u.UserID, u.Username,
		u.Email, u.Phone,
		u.PasswordHash, u.Salt)
	if err != nil {
		logHG.ErrFInfo("创建用户失败：", err)
		return err
	}
	_, err = stmt.LastInsertId()
	return err
}

/* 获取用户信息 */
func GetUserByEmail(ctx context.Context, account string) (*UserModelsPackage.HGUserModel, error) {
	HGLoggerPackage.LogInfo(ctx, map[string]any{
		"Tag":     HGLoggerPackage.LoginLogBeforeDesc,
		"account": account,
	})
	row := db.QueryRow(SQLQueriesPackage.GetUserByEmailOrPhoneSQL, account, account)

	u := &UserModelsPackage.HGUserModel{}
	err := row.Scan(&u.UserID, &u.Username, &u.Email, &u.Phone,
		&u.PasswordHash, &u.Salt)
	if err != nil {
		return nil, err
	}
	HGLoggerPackage.LogInfo(ctx, map[string]any{
		"Tag":       HGLoggerPackage.LoginLogAfterDesc,
		"user_info": u,
	})
	return u, nil
}
