package ConfigPackage

import (
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestGetInteractionReprojectConfigValidatesBounds(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("interaction_reproject.enabled", true)
	viper.Set("interaction_reproject.interval", "1m")
	viper.Set("interaction_reproject.timeout", "20s")
	viper.Set("interaction_reproject.safety_lag", "5s")
	viper.Set("interaction_reproject.lease_ttl", "30s")
	viper.Set("interaction_reproject.page_size", 500)
	viper.Set("interaction_reproject.worker_count", 2)
	viper.Set("interaction_reproject.hash_ranges", []map[string]any{{"start": 0, "end": 512}, {"start": 512, "end": 1024}})

	cfg, err := GetInteractionReprojectConfig()
	if err != nil {
		t.Fatalf("GetInteractionReprojectConfig() error = %v", err)
	}
	if !cfg.Enabled || cfg.Interval != time.Minute || cfg.PageSize != 500 || cfg.WorkerCount != 2 || len(cfg.HashRanges) != 2 || cfg.HashRanges[1].Start != 512 || cfg.HashRanges[1].End != 1024 {
		t.Fatalf("config = %+v", cfg)
	}

	viper.Set("interaction_reproject.page_size", 1001)
	if _, err := GetInteractionReprojectConfig(); err == nil {
		t.Fatal("expected oversized page to be rejected")
	}
}
