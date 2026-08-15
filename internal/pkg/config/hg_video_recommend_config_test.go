package ConfigPackage

import (
	"testing"

	"github.com/spf13/viper"
)

func TestGetVideoRecommendConfig(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("video_recommend.redis_generation", "v2")
	viper.Set("video_recommend.redis_shard_count", 64)
	viper.Set("video_recommend.redis_max_items_per_shard", 2000)
	cfg, err := GetVideoRecommendConfig()
	if err != nil {
		t.Fatalf("GetVideoRecommendConfig() error = %v", err)
	}
	if cfg.RedisGeneration != "v2" || cfg.RedisShardCount != 64 || cfg.RedisMaxItems != 2000 {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}
