package migrations_test

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestVideoCommentMigration21ContainsRepliesReactionsImagesAndShardedStats(t *testing.T) {
	data, err := os.ReadFile("000021_extend_video_comments.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(data)
	for _, fragment := range []string{
		"dislike_count", "image_urls", "video_comment_reactions", "UNIQUE KEY", "video_comment_stat_shards",
		"CRC32", "% 32", "submission_id`, `root_comment_id`, `is_deleted`, `created_at`, `id",
	} {
		if !strings.Contains(sql, fragment) {
			t.Fatalf("migration missing %q", fragment)
		}
	}
}

func TestVideoCommentMigration21DoesNotSelectHardCodedDatabase(t *testing.T) {
	for _, name := range []string{"000021_extend_video_comments.up.sql", "000021_extend_video_comments.down.sql"} {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if regexp.MustCompile(`(?im)^\s*USE\s+`).Match(data) {
			t.Fatalf("%s must use the migrator-selected database", name)
		}
	}
}
