package main

import (
	PersistenceSQLPackage "MLC_GO/internal/pkg/mysql"
	"io"
	"strings"
	"testing"
	"time"
)

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
