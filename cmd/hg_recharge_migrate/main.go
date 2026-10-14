package main

import (
	hgwalletrepo "MLC_GO/internal/modules/wallet/repository"
	ConfigPackage "MLC_GO/internal/pkg/config"
	PersistenceSQLPackage "MLC_GO/internal/pkg/mysql"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"
)

const hgDefaultMigrationTimeout = 10 * time.Second

type hgMigrationOptions struct {
	hgEnv                  string
	hgConfigDir            string
	hgConfirmStoppedWrites bool
	hgLow                  uint64
	hgHigh                 uint64
	hgTimeout              time.Duration
	hgLowSet, hgHighSet    bool
}

type hgMigrationDBOpener func() (*PersistenceSQLPackage.HGSQLManager, error)

func main() {
	if hgErr := hgRun(os.Args[1:], os.Stdout, hgOpenSQLManager); hgErr != nil {
		fmt.Fprintln(os.Stderr, hgErr)
		os.Exit(1)
	}
}

// hgRun 只执行一个显式授权的主键区间；缺少任一安全参数时在连接数据库前退出。
func hgRun(hgArgs []string, hgOut io.Writer, hgOpen hgMigrationDBOpener) error {
	hgOptions, hgErr := hgParseOptions(hgArgs, hgOut)
	if hgErr != nil {
		return hgErr
	}
	if hgOptions.hgEnv == "" || !hgOptions.hgConfirmStoppedWrites || !hgOptions.hgLowSet || !hgOptions.hgHighSet {
		return errors.New("默认不执行迁移；必须同时提供 --env debug|pre|prod、--confirm-stopped-writes、--low 和 --high")
	}
	if hgOptions.hgEnv != string(ConfigPackage.EnvDebug) && hgOptions.hgEnv != string(ConfigPackage.EnvPre) && hgOptions.hgEnv != string(ConfigPackage.EnvProd) {
		return fmt.Errorf("--env 只支持 debug、pre、prod，收到 %q", hgOptions.hgEnv)
	}
	if hgOptions.hgLow == 0 || hgOptions.hgHigh < hgOptions.hgLow || hgOptions.hgHigh-hgOptions.hgLow >= 500 {
		return errors.New("主键区间无效：要求 low > 0、high >= low 且最多500行")
	}
	if hgOptions.hgTimeout <= 0 {
		return errors.New("--timeout 必须大于0")
	}
	if err := os.Setenv("MLC_CONFIG_DIR", hgOptions.hgConfigDir); err != nil {
		return fmt.Errorf("设置配置目录失败: %w", err)
	}
	if err := os.Setenv("SERVER_ENV", hgOptions.hgEnv); err != nil {
		return fmt.Errorf("设置运行环境失败: %w", err)
	}
	if err := ConfigPackage.LoadConfig(hgOptions.hgEnv); err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}
	hgSQLManager, hgErr := hgOpen()
	if hgErr != nil {
		return fmt.Errorf("打开MySQL失败: %w", hgErr)
	}
	defer hgSQLManager.Close()
	hgCtx, hgCancel := context.WithTimeout(context.Background(), hgOptions.hgTimeout)
	defer hgCancel()
	hgCount, hgErr := hgwalletrepo.HGNewRepository(hgSQLManager.GetSQLDB()).HGMigrateLegacyRange(hgCtx, hgOptions.hgLow, hgOptions.hgHigh)
	if hgErr != nil {
		return fmt.Errorf("迁移主键区间[%d,%d]失败: %w", hgOptions.hgLow, hgOptions.hgHigh, hgErr)
	}
	fmt.Fprintf(hgOut, "充值历史迁移完成：区间[%d,%d]，处理%d条\n", hgOptions.hgLow, hgOptions.hgHigh, hgCount)
	return nil
}

func hgParseOptions(hgArgs []string, hgOut io.Writer) (hgMigrationOptions, error) {
	hgFlags := flag.NewFlagSet("hg_recharge_migrate", flag.ContinueOnError)
	hgFlags.SetOutput(hgOut)
	hgEnv := hgFlags.String("env", "", "运行环境：debug、pre、prod（必填）")
	hgConfigDir := hgFlags.String("config-dir", "./config", "配置根目录")
	hgConfirm := hgFlags.Bool("confirm-stopped-writes", false, "确认旧wallet写入和debug支付已停止（必填）")
	hgLowText := hgFlags.String("low", "", "旧表主键区间下界（必填）")
	hgHighText := hgFlags.String("high", "", "旧表主键区间上界（必填）")
	hgTimeout := hgFlags.Duration("timeout", hgDefaultMigrationTimeout, "单批迁移超时，默认10s")
	if hgErr := hgFlags.Parse(hgArgs); hgErr != nil {
		return hgMigrationOptions{}, hgErr
	}
	hgOptions := hgMigrationOptions{hgEnv: *hgEnv, hgConfigDir: *hgConfigDir, hgConfirmStoppedWrites: *hgConfirm, hgTimeout: *hgTimeout, hgLowSet: *hgLowText != "", hgHighSet: *hgHighText != ""}
	if hgOptions.hgLowSet {
		hgOptions.hgLow, _ = strconv.ParseUint(*hgLowText, 10, 64)
		if _, hgErr := strconv.ParseUint(*hgLowText, 10, 64); hgErr != nil {
			return hgMigrationOptions{}, fmt.Errorf("--low 必须是无符号整数: %w", hgErr)
		}
	}
	if hgOptions.hgHighSet {
		hgOptions.hgHigh, _ = strconv.ParseUint(*hgHighText, 10, 64)
		if _, hgErr := strconv.ParseUint(*hgHighText, 10, 64); hgErr != nil {
			return hgMigrationOptions{}, fmt.Errorf("--high 必须是无符号整数: %w", hgErr)
		}
	}
	return hgOptions, nil
}

func hgOpenSQLManager() (*PersistenceSQLPackage.HGSQLManager, error) {
	return PersistenceSQLPackage.NewSQLManager()
}
