package main

import (
	PersistenceSQLPackage "MLC_GO/internal/pkg/mysql"
	"bytes"
	"io"
	"strings"
	"testing"
	"time"
)

// TestHGRunHelp 验证终端及调试启动使用的帮助参数正常退出，且不打开数据库。
func TestHGRunHelp(t *testing.T) {
	for _, hgArg := range []string{"-h", "--help"} {
		t.Run(hgArg, func(t *testing.T) {
			var hgOut bytes.Buffer
			hgOpen := func() (*PersistenceSQLPackage.HGSQLManager, error) {
				t.Fatal("帮助模式不应打开数据库")
				return nil, nil
			}
			if hgErr := hgRun([]string{hgArg}, &hgOut, hgOpen); hgErr != nil {
				t.Fatalf("帮助模式应正常退出: %v", hgErr)
			}
			for _, hgText := range []string{"Usage of hg_recharge_migrate", "-env", "-confirm-stopped-writes", "-low", "-high"} {
				if !strings.Contains(hgOut.String(), hgText) {
					t.Fatalf("帮助信息缺少 %q", hgText)
				}
			}
		})
	}
}

func TestHGRunRequiresExplicitWriteAuthorization(t *testing.T) {
	hgOpened := false
	hgOpen := func() (*PersistenceSQLPackage.HGSQLManager, error) {
		hgOpened = true
		return nil, nil
	}
	// 使用类型适配函数，验证安全参数不足时不会初始化配置或数据库。
	hgErr := hgRun([]string{"--env", "debug", "--low", "1", "--high", "2"}, io.Discard, hgOpen)
	if hgErr == nil || !strings.Contains(hgErr.Error(), "默认不执行") {
		t.Fatalf("err=%v", hgErr)
	}
	if hgOpened {
		t.Fatal("database opener was called")
	}
}

func TestHGParseOptionsRequiresBoundedRangeAndDefaultTimeout(t *testing.T) {
	hgOptions, hgErr := hgParseOptions([]string{"--env", "pre", "--confirm-stopped-writes", "--low", "10", "--high", "509"}, io.Discard)
	if hgErr != nil {
		t.Fatal(hgErr)
	}
	if hgOptions.hgLow != 10 || hgOptions.hgHigh != 509 || hgOptions.hgTimeout != 10*time.Second {
		t.Fatalf("options=%+v", hgOptions)
	}
	if _, hgErr = hgParseOptions([]string{"--low", "x"}, io.Discard); hgErr == nil {
		t.Fatal("expected invalid low")
	}
}

func TestHGParseOptionsDoesNotAcceptEmptyEnvironmentAsExplicit(t *testing.T) {
	hgOptions, hgErr := hgParseOptions([]string{"--confirm-stopped-writes", "--low", "1", "--high", "1"}, io.Discard)
	if hgErr != nil {
		t.Fatal(hgErr)
	}
	if hgOptions.hgEnv != "" {
		t.Fatalf("env=%q", hgOptions.hgEnv)
	}
}
