package ConfigPackage

import (
	"testing"

	"github.com/spf13/viper"
)

func TestGetCrawlerConfigDisabled(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	t.Setenv("CRAWLER_BILIBILI_ENABLED", "false")
	viper.Set("crawler.bilibili.enabled", false)
	cfg, err := GetCrawlerConfig()
	if err != nil {
		t.Fatalf("GetCrawlerConfig() error = %v", err)
	}
	if cfg.Enabled {
		t.Fatal("GetCrawlerConfig() Enabled = true, want false")
	}
}

func TestGetCrawlerConfigEnabled(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	t.Setenv("CRAWLER_BILIBILI_ENABLED", "true")
	viper.Set("crawler.bilibili.enabled", true)
	viper.Set("crawler.bilibili.interval", "5m")
	viper.Set("crawler.bilibili.timeout", "10s")
	viper.Set("crawler.bilibili.max_items", 12)
	viper.Set("crawler.bilibili.retry_count", 2)
	viper.Set("crawler.bilibili.rate_per_second", 0.2)
	viper.Set("crawler.bilibili.user_agent", "MLC_GO-HGCrawler/1.0")
	cfg, err := GetCrawlerConfig()
	if err != nil {
		t.Fatalf("GetCrawlerConfig() error = %v", err)
	}
	if !cfg.Enabled || cfg.MaxItems != 12 || cfg.RetryCount != 2 {
		t.Fatalf("GetCrawlerConfig() = %+v", cfg)
	}
}
