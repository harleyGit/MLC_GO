package ConfigPackage

import (
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestPowerOfTwoResourceBoundary(t *testing.T) {
	for _, testCase := range []struct {
		value, minValue, maxValue int
		want                      bool
	}{
		{value: 16, minValue: 16, maxValue: 4096, want: true},
		{value: 256, minValue: 16, maxValue: 4096, want: true},
		{value: 15, minValue: 16, maxValue: 4096, want: false},
		{value: 48, minValue: 16, maxValue: 4096, want: false},
		{value: 8192, minValue: 16, maxValue: 4096, want: false},
	} {
		if got := hgPowerOfTwo(testCase.value, testCase.minValue, testCase.maxValue); got != testCase.want {
			t.Fatalf("hgPowerOfTwo(%d, %d, %d) = %t, want %t", testCase.value, testCase.minValue, testCase.maxValue, got, testCase.want)
		}
	}
}

func TestVideoDanmakuHeartbeatConfig(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("video_danmaku", map[string]any{
		"host":                    "0.0.0.0",
		"port":                    "8081",
		"allowed_origins":         []string{"http://localhost:5174"},
		"ticket_ttl":              "45s",
		"heartbeat_interval":      "20s",
		"heartbeat_timeout":       "60s",
		"drain_timeout":           "30s",
		"heartbeat_shard_count":   64,
		"worker_count":            16,
		"queue_size":              65536,
		"max_connections":         1000000,
		"max_frame_bytes":         4096,
		"max_handshake_bytes":     8192,
		"room_shard_count":        256,
		"member_shard_count":      64,
		"max_pending_bytes":       65536,
		"command_rate_per_second": 5,
		"command_burst":           10,
		"broadcast_worker_count":  64,
		"broadcast_queue_size":    4096,
		"recent_message_limit":    1000,
	})

	config, err := GetVideoDanmakuConfig()
	if err != nil {
		t.Fatalf("GetVideoDanmakuConfig() error = %v", err)
	}
	if config.HeartbeatInterval != 20*time.Second || config.HeartbeatTimeout != 60*time.Second || config.DrainTimeout != 30*time.Second || config.HeartbeatShardCount != 64 {
		t.Fatalf("heartbeat/drain config = interval %s timeout %s drain %s shards %d", config.HeartbeatInterval, config.HeartbeatTimeout, config.DrainTimeout, config.HeartbeatShardCount)
	}
}

func TestVideoDanmakuDefaultsMissingDrainTimeout(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("video_danmaku", map[string]any{
		"host": "0.0.0.0", "port": "8081", "allowed_origins": []string{"http://localhost:5174"},
		"ticket_ttl": "45s", "heartbeat_interval": "20s", "heartbeat_timeout": "60s", "heartbeat_shard_count": 64,
		"worker_count": 16, "queue_size": 65536, "max_connections": 1000000, "max_frame_bytes": 4096,
		"max_handshake_bytes": 8192, "room_shard_count": 256, "member_shard_count": 64, "max_pending_bytes": 65536,
		"command_rate_per_second": 5, "command_burst": 10, "broadcast_worker_count": 64,
		"broadcast_queue_size": 4096, "recent_message_limit": 1000,
	})
	config, err := GetVideoDanmakuConfig()
	if err != nil {
		t.Fatalf("GetVideoDanmakuConfig() missing drain timeout error = %v", err)
	}
	if config.DrainTimeout != 30*time.Second {
		t.Fatalf("default drain timeout = %s, want 30s", config.DrainTimeout)
	}
}

func TestVideoDanmakuRejectsInvalidHeartbeatShardCount(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("video_danmaku", map[string]any{
		"host": "0.0.0.0", "port": "8081", "allowed_origins": []string{"http://localhost:5174"},
		"ticket_ttl": "45s", "heartbeat_interval": "20s", "heartbeat_timeout": "60s", "drain_timeout": "60s", "heartbeat_shard_count": 48,
		"worker_count": 16, "queue_size": 65536, "max_connections": 1000000, "max_frame_bytes": 4096,
		"max_handshake_bytes": 8192, "room_shard_count": 256, "member_shard_count": 64, "max_pending_bytes": 65536,
		"command_rate_per_second": 5, "command_burst": 10, "broadcast_worker_count": 64,
		"broadcast_queue_size": 4096, "recent_message_limit": 1000,
	})
	if _, err := GetVideoDanmakuConfig(); err == nil {
		t.Fatal("GetVideoDanmakuConfig() accepted non-power-of-two heartbeat shards")
	}
}

func TestVideoDanmakuRejectsInvalidDrainTimeout(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("video_danmaku", map[string]any{
		"host": "0.0.0.0", "port": "8081", "allowed_origins": []string{"http://localhost:5174"},
		"ticket_ttl": "45s", "heartbeat_interval": "20s", "heartbeat_timeout": "60s", "drain_timeout": "3s", "heartbeat_shard_count": 64,
		"worker_count": 16, "queue_size": 65536, "max_connections": 1000000, "max_frame_bytes": 4096,
		"max_handshake_bytes": 8192, "room_shard_count": 256, "member_shard_count": 64, "max_pending_bytes": 65536,
		"command_rate_per_second": 5, "command_burst": 10, "broadcast_worker_count": 64,
		"broadcast_queue_size": 4096, "recent_message_limit": 1000,
	})
	if _, err := GetVideoDanmakuConfig(); err == nil {
		t.Fatal("GetVideoDanmakuConfig() accepted drain timeout below 5s")
	}
}
