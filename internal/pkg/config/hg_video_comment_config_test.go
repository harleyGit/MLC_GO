package ConfigPackage

import (
	"net/netip"
	"testing"

	"github.com/spf13/viper"
)

func TestGetVideoCommentConfigRequiresS3SecretsFromEnvironment(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("video_comment.storage.type", "s3")
	viper.Set("video_comment.storage.endpoint", "https://s3.example.com")
	viper.Set("video_comment.storage.region", "ap-southeast-1")
	viper.Set("video_comment.storage.bucket", "comment-images")
	viper.Set("video_comment.storage.cdn_base_url", "https://cdn.example.com")
	viper.Set("video_comment.storage.request_timeout", "10s")
	viper.Set("video_comment.image.user_capacity_bytes", int64(100<<20))
	viper.Set("video_comment.image.rate_user_capacity", 6)
	viper.Set("video_comment.image.rate_ip_capacity", 30)
	viper.Set("video_comment.image.rate_window", "1m")
	viper.Set("video_comment.maintenance.enabled", true)
	viper.Set("video_comment.maintenance.interval", "1m")
	viper.Set("video_comment.maintenance.timeout", "20s")
	viper.Set("video_comment.maintenance.orphan_age", "24h")
	viper.Set("video_comment.maintenance.batch_size", 100)

	if _, err := GetVideoCommentConfig(); err == nil {
		t.Fatal("GetVideoCommentConfig() expected missing S3 credential error")
	}
	t.Setenv("VIDEO_COMMENT_S3_ACCESS_KEY_ID", "access")
	t.Setenv("VIDEO_COMMENT_S3_SECRET_ACCESS_KEY", "secret")
	cfg, err := GetVideoCommentConfig()
	if err != nil {
		t.Fatalf("GetVideoCommentConfig() error=%v", err)
	}
	if cfg.Storage.CDNBaseURL != "https://cdn.example.com" || cfg.Image.UserCapacityBytes != 100<<20 {
		t.Fatalf("GetVideoCommentConfig() cfg=%+v", cfg)
	}
}

func TestGetVideoCommentConfigParsesTrustedProxyCIDRs(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("video_comment.storage.type", "local")
	viper.Set("video_comment.storage.request_timeout", "10s")
	viper.Set("video_comment.image.user_capacity_bytes", int64(100<<20))
	viper.Set("video_comment.image.rate_user_capacity", 6)
	viper.Set("video_comment.image.rate_ip_capacity", 30)
	viper.Set("video_comment.image.rate_window", "1m")
	viper.Set("video_comment.trusted_proxy_cidrs", []string{"10.0.0.0/8", "2001:db8::/32"})

	cfg, err := GetVideoCommentConfig()
	if err != nil {
		t.Fatalf("GetVideoCommentConfig() error=%v", err)
	}
	want := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("2001:db8::/32")}
	if len(cfg.TrustedProxyCIDRs) != len(want) || cfg.TrustedProxyCIDRs[0] != want[0] || cfg.TrustedProxyCIDRs[1] != want[1] {
		t.Fatalf("TrustedProxyCIDRs=%v, want %v", cfg.TrustedProxyCIDRs, want)
	}
}

func TestGetVideoCommentConfigRejectsInvalidTrustedProxyCIDR(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("video_comment.storage.type", "local")
	viper.Set("video_comment.storage.request_timeout", "10s")
	viper.Set("video_comment.image.user_capacity_bytes", int64(100<<20))
	viper.Set("video_comment.image.rate_user_capacity", 6)
	viper.Set("video_comment.image.rate_ip_capacity", 30)
	viper.Set("video_comment.image.rate_window", "1m")
	viper.Set("video_comment.trusted_proxy_cidrs", []string{"not-a-cidr"})

	if _, err := GetVideoCommentConfig(); err == nil {
		t.Fatal("GetVideoCommentConfig() expected invalid trusted proxy CIDR error")
	}
}

func TestGetVideoCommentConfigRejectsTrustingEveryAddress(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("video_comment.storage.type", "local")
	viper.Set("video_comment.storage.request_timeout", "10s")
	viper.Set("video_comment.image.user_capacity_bytes", int64(100<<20))
	viper.Set("video_comment.image.rate_user_capacity", 6)
	viper.Set("video_comment.image.rate_ip_capacity", 30)
	viper.Set("video_comment.image.rate_window", "1m")
	viper.Set("video_comment.trusted_proxy_cidrs", []string{"0.0.0.0/0"})

	if _, err := GetVideoCommentConfig(); err == nil {
		t.Fatal("GetVideoCommentConfig() expected unsafe trusted proxy CIDR error")
	}
}

func TestGetVideoCommentConfigAllowsTrustedProxyEnvironmentOverride(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("video_comment.storage.type", "local")
	viper.Set("video_comment.storage.request_timeout", "10s")
	viper.Set("video_comment.image.user_capacity_bytes", int64(100<<20))
	viper.Set("video_comment.image.rate_user_capacity", 6)
	viper.Set("video_comment.image.rate_ip_capacity", 30)
	viper.Set("video_comment.image.rate_window", "1m")
	viper.Set("video_comment.trusted_proxy_cidrs", []string{"10.0.0.0/8"})
	t.Setenv("VIDEO_COMMENT_TRUSTED_PROXY_CIDRS", "192.0.2.0/24, 2001:db8::/32")

	cfg, err := GetVideoCommentConfig()
	if err != nil {
		t.Fatalf("GetVideoCommentConfig() error=%v", err)
	}
	if len(cfg.TrustedProxyCIDRs) != 2 || cfg.TrustedProxyCIDRs[0] != netip.MustParsePrefix("192.0.2.0/24") {
		t.Fatalf("TrustedProxyCIDRs=%v", cfg.TrustedProxyCIDRs)
	}
}

func TestProductionVideoCommentConfigLoadsTrustedProxyCIDRs(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	// 使用仓库真实 config/base + config/prod 加载流程；S3 使用假值，仅用于通过配置完整性校验，不会发起网络请求。
	t.Setenv("MLC_CONFIG_DIR", projectConfigDir(t))
	t.Setenv("VIDEO_COMMENT_S3_ENDPOINT", "https://s3.example.com")
	t.Setenv("VIDEO_COMMENT_S3_REGION", "ap-southeast-1")
	t.Setenv("VIDEO_COMMENT_S3_BUCKET", "comment-images")
	t.Setenv("VIDEO_COMMENT_CDN_BASE_URL", "https://cdn.example.com")
	t.Setenv("VIDEO_COMMENT_S3_ACCESS_KEY_ID", "access")
	t.Setenv("VIDEO_COMMENT_S3_SECRET_ACCESS_KEY", "secret")

	if err := LoadConfig("prod"); err != nil {
		t.Fatalf("LoadConfig(prod) error=%v", err)
	}
	cfg, err := GetVideoCommentConfig()
	if err != nil {
		t.Fatalf("GetVideoCommentConfig() error=%v", err)
	}
	// 确认生产 YAML 中的两个代理网段都被解析成规范 CIDR，避免只验证 YAML 语法却没有验证程序实际读取结果。
	want := []netip.Prefix{netip.MustParsePrefix("10.20.0.0/16"), netip.MustParsePrefix("10.30.0.0/16")}
	if len(cfg.TrustedProxyCIDRs) != len(want) || cfg.TrustedProxyCIDRs[0] != want[0] || cfg.TrustedProxyCIDRs[1] != want[1] {
		t.Fatalf("TrustedProxyCIDRs=%v, want %v", cfg.TrustedProxyCIDRs, want)
	}
}
