package ConfigPackage

import (
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestGetCoinJobConfigValidatesBounds(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("coin_jobs.enabled", true)
	viper.Set("coin_jobs.interval", "1m")
	viper.Set("coin_jobs.timeout", "20s")
	viper.Set("coin_jobs.batch_size", 200)

	cfg, err := GetCoinJobConfig()
	if err != nil {
		t.Fatalf("GetCoinJobConfig() error = %v", err)
	}
	if cfg.Interval != time.Minute || cfg.Timeout != 20*time.Second || cfg.BatchSize != 200 {
		t.Fatalf("config = %+v", cfg)
	}
	viper.Set("coin_jobs.batch_size", 1001)
	if _, err := GetCoinJobConfig(); err == nil {
		t.Fatal("expected oversized batch to be rejected")
	}
}
