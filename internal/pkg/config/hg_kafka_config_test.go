package ConfigPackage

import (
	HGKafkaPackage "MLC_GO/internal/pkg/kafka"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestGetKafkaConfigRejectsEmptyBusinessBrokers(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	_, enabled, err := GetKafkaConfig()
	if err == nil {
		t.Fatal("expected empty business kafka brokers to be rejected")
	}
	if enabled {
		t.Fatal("expected kafka disabled when required business brokers are invalid")
	}
	if !strings.Contains(err.Error(), "business.brokers") {
		t.Fatalf("expected missing business.brokers error, got %v", err)
	}
}

func TestKafkaConfiguredForEveryEnvironment(t *testing.T) {
	for _, env := range []string{"debug", "pre", "prod"} {
		t.Run(env, func(t *testing.T) {
			configPath := filepath.Join("..", "..", "..", "config", "config."+env+".yaml")
			configReader := viper.New()
			configReader.SetConfigFile(configPath)
			if err := configReader.ReadInConfig(); err != nil {
				t.Fatalf("read %s config: %v", env, err)
			}

			var cfg HGKafkaPackage.HGKafkaClusterConfig
			if err := configReader.UnmarshalKey("kafka", &cfg); err != nil {
				t.Fatalf("unmarshal %s kafka config: %v", env, err)
			}
			if _, err := HGKafkaPackage.HGBuildClusterConfig(cfg.Business); err != nil {
				t.Fatalf("invalid %s business kafka config: %v", env, err)
			}
			if _, err := HGKafkaPackage.HGBuildClusterConfig(cfg.Log); err != nil {
				t.Fatalf("invalid %s log kafka config: %v", env, err)
			}
		})
	}
}
