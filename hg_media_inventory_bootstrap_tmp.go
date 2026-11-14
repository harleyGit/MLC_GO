// 本机 debug 一次性 IAM 引导；所有写入须在同一事务内完成，执行后移除本文件。
package main

import (
	Config "MLC_GO/internal/pkg/config"
	Queries "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/go-sql-driver/mysql"
)

const (
	hgTargetUser = "hgid_B1ZJ-75K5-20T9-D1TF"
	hgRoleCode   = "media_inventory_reader_debug"
	hgPermission = "media:inventory:read"
	hgActor      = "local-debug-bootstrap"
	hgRequestID  = "local-debug-media-inventory-bootstrap-20261114"
)

func hgMust(err error, step string) {
	if err != nil {
		fmt.Fprintln(os.Stderr, step+"失败："+err.Error())
		os.Exit(1)
	}
}

func hgRun() error {
	if os.Getenv("SERVER_ENV") != "debug" || os.Getenv("MLC_CONFIG_DIR") != "" {
		return errors.New("仅允许从项目根使用默认配置目录和 SERVER_ENV=debug")
	}
	if err := Config.LoadConfig("debug"); err != nil {
		return errors.New("无法加载本机 debug 配置")
	}
	cfg, err := Config.GetMySQLConfig()
	if err != nil || cfg.Host != "127.0.0.1" || cfg.Port != "3306" || cfg.Database != "HG_MLC_DB" {
		return errors.New("拒绝连接非本机 debug 数据库")
	}
	dsn := (&mysql.Config{User: cfg.User, Passwd: cfg.Password, Net: "tcp", Addr: net.JoinHostPort(cfg.Host, cfg.Port), DBName: cfg.Database, Timeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second}).FormatDSN()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return errors.New("连接本机数据库失败")
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var database string
	if err = db.QueryRowContext(ctx, Queries.HGSelectLocalMediaBootstrapDatabase).Scan(&database); err != nil || database != "HG_MLC_DB" {
		return errors.New("数据库名与预期不符")
	}
	var one int
	if err = db.QueryRowContext(ctx, Queries.HGSelectIAMManager, hgTargetUser).Scan(&one); err != nil || one != 1 {
		return errors.New("目标不是本机 debug 的活动管理员")
	}
	var permissionID uint64
	if err = db.QueryRowContext(ctx, Queries.HGSelectLocalMediaBootstrapPermission, hgPermission).Scan(&permissionID); err != nil {
		return errors.New("缺少媒体盘点权限目录；停止引导")
	}
	var existingRole uint64
	err = db.QueryRowContext(ctx, Queries.HGSelectLocalMediaBootstrapRole, hgRoleCode).Scan(&existingRole)
	if err == nil || !errors.Is(err, sql.ErrNoRows) {
		return errors.New("专用角色已存在或无法检查，停止重复引导")
	}
	err = db.QueryRowContext(ctx, Queries.HGSelectIAMPermission, hgTargetUser, "global", "", hgPermission).Scan(&one)
	if err == nil || !errors.Is(err, sql.ErrNoRows) {
		return errors.New("目标已持有该权限或权限检查失败，停止重复引导")
	}

	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return errors.New("无法开启受控事务")
	}
	defer tx.Rollback()
	// 单次事务内只创建一个专用角色、一条权限映射、一条全局绑定和两条审计。
	res, err := tx.ExecContext(ctx, Queries.HGInsertIAMRole, hgRoleCode, "本机媒体盘点", "仅本机 debug 指定管理员进行新旧媒体只读元数据盘点")
	if err != nil {
		return errors.New("创建专用角色失败")
	}
	roleID, err := res.LastInsertId()
	if err != nil {
		return errors.New("读取角色 ID 失败")
	}
	res, err = tx.ExecContext(ctx, Queries.HGInsertIAMRolePermission, roleID, hgPermission)
	if err != nil {
		return errors.New("授予专用角色权限失败")
	}
	n, err := res.RowsAffected()
	if err != nil || n != 1 {
		return errors.New("权限映射数不符")
	}
	res, err = tx.ExecContext(ctx, Queries.HGInsertIAMBinding, hgTargetUser, roleID, "global", "", nil, hgActor)
	if err != nil {
		return errors.New("绑定目标账号失败")
	}
	n, err = res.RowsAffected()
	if err != nil || n != 1 {
		return errors.New("绑定行数不符")
	}
	res, err = tx.ExecContext(ctx, Queries.HGBumpIAMPolicyRevision)
	if err != nil {
		return errors.New("推进 IAM 策略版本失败")
	}
	n, err = res.RowsAffected()
	if err != nil || n != 1 {
		return errors.New("策略版本更新行数不符")
	}
	for _, audit := range []struct{ action, kind, target, detail string }{
		{"iam.role.create", "role", fmt.Sprint(roleID), `{"roleCode":"media_inventory_reader_debug","reason":"local_debug_bootstrap"}`},
		{"iam.binding.grant", "user", hgTargetUser, `{"roleCode":"media_inventory_reader_debug","scopeType":"global","scopeId":"","reason":"local_debug_bootstrap"}`},
	} {
		res, err = tx.ExecContext(ctx, Queries.HGInsertIAMAudit, hgActor, audit.action, audit.kind, audit.target, hgRequestID, audit.detail)
		if err != nil {
			return errors.New("写入 IAM 审计失败")
		}
		n, err = res.RowsAffected()
		if err != nil || n != 1 {
			return errors.New("审计行数不符")
		}
	}
	if err = tx.Commit(); err != nil {
		return errors.New("提交状态不确定，请只读检查后再处理")
	}
	var allowed, audited, mapped, bound int
	if err = db.QueryRowContext(ctx, Queries.HGSelectIAMPermission, hgTargetUser, "global", "", hgPermission).Scan(&allowed); err != nil || allowed != 1 {
		return errors.New("已提交，但只读核对目标权限失败")
	}
	if err = db.QueryRowContext(ctx, Queries.HGSelectLocalMediaBootstrapRolePermissions, roleID).Scan(&mapped); err != nil || mapped != 1 {
		return errors.New("已提交，但专用角色权限数量核对失败")
	}
	if err = db.QueryRowContext(ctx, Queries.HGSelectLocalMediaBootstrapRoleBindings, roleID).Scan(&bound); err != nil || bound != 1 {
		return errors.New("已提交，但专用角色绑定数量核对失败")
	}
	if err = db.QueryRowContext(ctx, Queries.HGSelectLocalMediaBootstrapAudit, hgRequestID).Scan(&audited); err != nil || audited != 2 {
		return errors.New("已提交，但只读核对审计失败")
	}
	fmt.Printf("本机 debug 引导成功：专用角色 ID=%d，权限仅绑定指定账号，全局作用域；两条审计记录已核对。\n", roleID)
	return nil
}

func main() {
	hgMust(hgRun(), "媒体盘点授权引导")
}
