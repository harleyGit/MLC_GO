package migrations

import (
	"os"
	"strings"
	"testing"
)

func TestVideoDanmakuMigrationHasTimelineAndShardedCounter(t *testing.T) {
	content, err := os.ReadFile("000024_create_video_danmaku.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(content)
	for _, required := range []string{"idx_video_danmaku_timeline", "(`video_id`, `status`, `progress_ms`, `id`)", "video_danmaku_stat_shards", "CHECK (`shard_id` < 64)", "uk_video_danmaku_user_request"} {
		if !strings.Contains(sql, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
	if strings.Contains(strings.ToUpper(sql), "USE ") {
		t.Fatal("migration must use the database selected by the migrator")
	}
	down, err := os.ReadFile("000024_create_video_danmaku.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToUpper(string(down)), "USE ") {
		t.Fatal("down migration must use the database selected by the migrator")
	}
}
