//go:build production

package main

import (
	"testing"

	"github.com/spf13/viper"
)

// TestHGCommerceDebugTradingOnlyBindsLoopback 防止误将已开启模拟收款的业务入口监听到所有网卡。
func TestHGCommerceDebugTradingOnlyBindsLoopback(t *testing.T) {
	previous := viper.Get("runtime.loaded_env")
	t.Cleanup(func() { viper.Set("runtime.loaded_env", previous) })
	for _, tc := range []struct {
		process string // 进程环境；只有严格 debug 才可能开放模拟交易。
		loaded  string // 实际加载的配置环境；必须与进程环境一致。
		flag    string // 显式交易开关，仅精确 true 有效。
		port    string // 配置端口，兼容有无前导冒号。
		want    string // 期待的业务 HTTP 监听地址。
	}{
		{"debug", "debug", "true", "8080", "127.0.0.1:8080"},
		{"debug", "debug", "true", ":8080", "127.0.0.1:8080"},
		{"debug", "debug", "false", "8080", ":8080"},
		{"pre", "pre", "true", "8080", ":8080"},
		{"prod", "prod", "true", ":8080", ":8080"},
		{"debug", "pre", "true", "8080", ":8080"},
	} {
		t.Setenv("SERVER_ENV", tc.process)
		t.Setenv("MLC_COMMERCE_DEBUG_TRADING", tc.flag)
		viper.Set("runtime.loaded_env", tc.loaded)
		if got := buildListenAddr(tc.port); got != tc.want {
			t.Fatalf("process=%s loaded=%s flag=%s: 监听地址=%q，期望=%q", tc.process, tc.loaded, tc.flag, got, tc.want)
		}
	}
}
