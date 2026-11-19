package main

import (
	ConfigPackage "MLC_GO/internal/pkg/config"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/go-sql-driver/mysql"
)

// hgCheckIAMScopeTypes 只读执行接口使用的字典查询及字段扫描，核对 000076 的索引；不读取用户绑定和凭据。
func hgCheckIAMScopeTypes(hgConfig ConfigPackage.HGMySQLConfig) error {
	hgDB, hgErr := sql.Open("mysql", (&mysql.Config{
		User: hgConfig.User, Passwd: hgConfig.Password, Net: "tcp",
		Addr: net.JoinHostPort(hgConfig.Host, hgConfig.Port), DBName: hgConfig.Database,
		Timeout: 3 * time.Second, ReadTimeout: 3 * time.Second,
	}).FormatDSN())
	if hgErr != nil {
		return fmt.Errorf("作用域字典检查连接配置无效")
	}
	defer hgDB.Close()
	hgCtx, hgCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer hgCancel()
	// 只输出非敏感目标，以免把另一个环境的检查结果当作当前服务的迁移证据。
	fmt.Printf("mysql_target=%s database=%s\n", net.JoinHostPort(hgConfig.Host, hgConfig.Port), hgConfig.Database)
	hgRows, hgErr := hgDB.QueryContext(hgCtx, SQLQueriesPackage.HGSelectIAMScopeTypesFirst, 51)
	if hgErr != nil {
		var hgMySQLError *mysql.MySQLError
		if errors.As(hgErr, &hgMySQLError) {
			if hgMySQLError.Number == 1146 {
				return fmt.Errorf("作用域类型表缺失（MySQL 1146），请核对并应用 000076_iam_scope_types 迁移")
			}
			return fmt.Errorf("作用域字典查询失败（MySQL %d）", hgMySQLError.Number)
		}
		return fmt.Errorf("作用域字典查询失败，请检查数据库连接和超时")
	}
	defer hgRows.Close()
	hgCount, hgActive := 0, 0
	for hgRows.Next() {
		var hgID uint64
		var hgCode, hgName, hgDescription string
		var hgRequiresScopeID bool
		var hgStatus int
		if hgErr = hgRows.Scan(&hgID, &hgCode, &hgName, &hgDescription, &hgRequiresScopeID, &hgStatus); hgErr != nil {
			return fmt.Errorf("作用域字典字段扫描失败，请核对表结构")
		}
		hgCount++
		if hgStatus == 1 {
			hgActive++
		}
	}
	if hgRows.Err() != nil {
		return fmt.Errorf("作用域字典结果读取失败")
	}
	if hgErr = hgRows.Close(); hgErr != nil {
		return fmt.Errorf("作用域字典查询结束失败")
	}
	var hgIndexColumns string
	if hgErr = hgDB.QueryRowContext(hgCtx, SQLQueriesPackage.HGSelectIAMScopeTypeIndexColumns).Scan(&hgIndexColumns); hgErr != nil {
		return fmt.Errorf("作用域类型绑定索引检查失败")
	}
	if hgIndexColumns != "scope_type,status,id" {
		return fmt.Errorf("作用域类型绑定索引缺失或不匹配，请核对 000076 迁移")
	}
	fmt.Printf("iam_scope_types_query=ok fetched=%d active_in_page=%d has_more=%t binding_scope_index=ok\n", hgCount, hgActive, hgCount > 50)
	return nil
}
