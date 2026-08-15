package ConfigPackage

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestGetAPIGatewayConfig(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	t.Setenv("API_GATEWAY_TRUSTED_PROXY_CIDRS", "")
	viper.Set("api_gateway.enabled", true)
	viper.Set("api_gateway.max_url_bytes", 8192)
	viper.Set("api_gateway.supported_versions", []string{"v1"})
	viper.Set("api_gateway.trusted_proxy_cidrs", []string{"10.20.0.0/16"})
	for _, module := range []string{"auth", "profile", "video_upload", "bilibili", "video_interaction", "video_comment", "video_danmaku", "ops"} {
		viper.Set("api_gateway.modules."+module+".capacity", 10)
		viper.Set("api_gateway.modules."+module+".refill_per_second", 2)
		viper.Set("api_gateway.modules."+module+".max_body_bytes", 1024)
		viper.Set("api_gateway.modules."+module+".max_in_flight", 10)
	}

	cfg, err := GetAPIGatewayConfig()
	if err != nil {
		t.Fatalf("GetAPIGatewayConfig() error = %v", err)
	}
	if !cfg.Enabled || cfg.MaxURLBytes != 8192 || len(cfg.SupportedVersions) != 1 || len(cfg.Modules) != 8 || len(cfg.TrustedProxyCIDRs) != 1 {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}

func TestGetAPIGatewayConfigRejectsUnsafeUpstreamURL(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	t.Setenv("API_GATEWAY_TRUSTED_PROXY_CIDRS", "")
	viper.Set("api_gateway.enabled", true)
	viper.Set("api_gateway.max_url_bytes", 8192)
	viper.Set("api_gateway.supported_versions", []string{"v1"})
	for _, module := range []string{"auth", "profile", "video_upload", "bilibili", "video_interaction", "video_comment", "video_danmaku", "ops"} {
		viper.Set("api_gateway.modules."+module+".capacity", 10)
		viper.Set("api_gateway.modules."+module+".refill_per_second", 2)
		viper.Set("api_gateway.modules."+module+".max_body_bytes", 1024)
		viper.Set("api_gateway.modules."+module+".max_in_flight", 10)
	}
	viper.Set("api_gateway.modules.auth.upstream_url", "https://user:secret@example.com/path")
	if _, err := GetAPIGatewayConfig(); err == nil || !strings.Contains(err.Error(), "upstream_url") {
		t.Fatalf("GetAPIGatewayConfig() error = %v", err)
	}
}

func TestGetAPIGatewayConfigRejectsTrustAllProxy(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("api_gateway.enabled", true)
	viper.Set("api_gateway.max_url_bytes", 8192)
	viper.Set("api_gateway.supported_versions", []string{"v1"})
	t.Setenv("API_GATEWAY_TRUSTED_PROXY_CIDRS", "0.0.0.0/0")
	if _, err := GetAPIGatewayConfig(); err == nil {
		t.Fatal("GetAPIGatewayConfig() accepted trust-all proxy CIDR")
	}
}
