package ConfigPackage

import (
	"reflect"
	"testing"
	"time"

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

func TestGetCrawlerTaskConfigWithEnvironmentOverrides(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("crawler.tasks.allowed_hosts", []string{"api.bilibili.com"})
	viper.Set("crawler.tasks.allow_http", false)
	viper.Set("crawler.tasks.scheduler_enabled", false)
	viper.Set("crawler.tasks.refresh_interval", "30s")
	viper.Set("crawler.tasks.max_tasks", 100)
	viper.Set("crawler.tasks.lease_grace", "20s")
	viper.Set("crawler.tasks.default_user_agent", "MLC crawler")
	t.Setenv("CRAWLER_TASK_ALLOWED_HOSTS", "Example.COM, api.example.com.")
	t.Setenv("CRAWLER_TASK_ALLOW_HTTP", "true")
	t.Setenv("CRAWLER_TASK_SCHEDULER_ENABLED", "true")

	cfg, err := GetCrawlerTaskConfig()
	if err != nil {
		t.Fatalf("GetCrawlerTaskConfig() error = %v", err)
	}
	if !reflect.DeepEqual(cfg.AllowedHosts, []string{"example.com", "api.example.com"}) || !cfg.AllowHTTP || !cfg.SchedulerEnabled {
		t.Fatalf("GetCrawlerTaskConfig() = %+v", cfg)
	}
	if cfg.RefreshInterval != 30*time.Second || cfg.LeaseGrace != 20*time.Second || cfg.MaxTasks != 100 {
		t.Fatalf("GetCrawlerTaskConfig() bounds = %+v", cfg)
	}
}

func TestGetCrawlerTaskConfigRejectsEmptyAllowlist(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("crawler.tasks.refresh_interval", "30s")
	viper.Set("crawler.tasks.max_tasks", 100)
	viper.Set("crawler.tasks.lease_grace", "20s")
	viper.Set("crawler.tasks.default_user_agent", "MLC crawler")
	if _, err := GetCrawlerTaskConfig(); err == nil {
		t.Fatal("GetCrawlerTaskConfig() accepted an empty host allowlist")
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
