package ConfigPackage

import (
	"testing"

	"github.com/spf13/viper"
)

func TestGetAPIGatewayConfig(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	t.Setenv("API_GATEWAY_TRUSTED_PROXY_CIDRS", "")
	viper.Set("api_gateway.enabled", true)
	viper.Set("api_gateway.trusted_proxy_cidrs", []string{"10.20.0.0/16"})
	for _, module := range []string{"auth", "profile", "video_upload", "bilibili", "video_interaction", "video_comment", "video_danmaku", "ops"} {
		viper.Set("api_gateway.modules."+module+".capacity", 10)
		viper.Set("api_gateway.modules."+module+".refill_per_second", 2)
	}

	cfg, err := GetAPIGatewayConfig()
	if err != nil {
		t.Fatalf("GetAPIGatewayConfig() error = %v", err)
	}
	if !cfg.Enabled || len(cfg.Modules) != 8 || len(cfg.TrustedProxyCIDRs) != 1 {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}

func TestGetAPIGatewayConfigRejectsTrustAllProxy(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("api_gateway.enabled", true)
	t.Setenv("API_GATEWAY_TRUSTED_PROXY_CIDRS", "0.0.0.0/0")
	if _, err := GetAPIGatewayConfig(); err == nil {
		t.Fatal("GetAPIGatewayConfig() accepted trust-all proxy CIDR")
	}
}
