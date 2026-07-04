package ConfigPackage

import "testing"

func TestGetKafkaConfigReturnsDisabledWhenBrokersEmpty(t *testing.T) {
	cfg, enabled, err := GetKafkaConfig()
	if err != nil {
		t.Fatalf("expected no error for empty kafka config, got %v", err)
	}

	if enabled {
		t.Fatal("expected kafka disabled when brokers are empty")
	}

	if len(cfg.Business.Brokers) != 0 || len(cfg.Log.Brokers) != 0 {
		t.Fatalf("expected empty brokers, got business=%v log=%v", cfg.Business.Brokers, cfg.Log.Brokers)
	}
}
