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

func TestVideoCommentMigration22AddsShardedReactionsAndImageLifecycle(t *testing.T) {
	data, err := os.ReadFile("000022_productionize_video_comments.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(data)
	for _, fragment := range []string{"video_comment_reaction_shards", "video_comment_reaction_dirty", "video_comment_images", "video_comment_image_quotas", "idx_video_comment_images_cleanup"} {
		if !strings.Contains(sql, fragment) {
			t.Fatalf("migration missing %q", fragment)
		}
	}
	if regexp.MustCompile(`(?im)^\s*USE\s+`).Match(data) {
		t.Fatal("migration must use the migrator-selected database")
	}
}

func TestVideoCommentSchemaMigrationsDoNotRunUnboundedReactionBackfill(t *testing.T) {
	data, err := os.ReadFile("000022_productionize_video_comments.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if regexp.MustCompile(`(?is)INSERT\s+INTO\s+` + "`?video_comment_reaction_shards`?" + `.*SELECT.*FROM\s+` + "`?video_comment_reactions`?").Match(data) {
		t.Fatal("schema migration must not run an unbounded reaction backfill")
	}
}

func TestVideoCommentMigration23AddsRecoverableReservationsAndDirtyRevision(t *testing.T) {
	data, err := os.ReadFile("000023_recover_video_comment_maintenance.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	for _, fragment := range []string{"revision", "cleanup_token", "cleanup_lease_until", "idx_video_comment_images_pending_cleanup", "idx_video_comment_images_deleting_lease", "video_comment_reaction_backfill_state", "EXISTS (SELECT 1 FROM `video_comment_reaction_shards` LIMIT 1)"} {
		if !strings.Contains(string(data), fragment) {
			t.Fatalf("migration missing %q", fragment)
		}
	}
}

func TestVideoCommentMigration23DownRequeuesDeletingAssets(t *testing.T) {
	data, err := os.ReadFile("000023_recover_video_comment_maintenance.down.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if !strings.Contains(string(data), "SET `status` = 'delete_pending'") {
		t.Fatal("down migration must requeue deleting assets before dropping cleanup lease columns")
	}
	if strings.Contains(string(data), "DROP TABLE IF EXISTS `video_comment_reaction_backfill_state`") {
		t.Fatal("down migration must retain the backfill checkpoint across rollback and re-upgrade")
	}
}

func TestVideoCommentMigration27AddsReplyNameSnapshotWithoutUnboundedBackfill(t *testing.T) {
	data, err := os.ReadFile("000027_add_video_comment_reply_name_snapshot.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(data)
	for _, fragment := range []string{"reply_to_user_name", "video_comment_reply_shards", "shard_id", "< 256", "video_comment_reply_dirty", "revision"} {
		if !strings.Contains(sql, fragment) {
			t.Fatalf("migration missing %q", fragment)
		}
	}
	if regexp.MustCompile(`(?is)UPDATE\s+` + "`?video_comments`?" + `|INSERT\s+INTO\s+` + "`?video_comments`?").Match(data) {
		t.Fatal("schema migration must not run an unbounded video_comments backfill")
	}
}
