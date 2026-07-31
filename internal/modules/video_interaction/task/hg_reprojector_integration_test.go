package VideoInteractionTaskPackage

import (
	VideoInteractionCachePackage "MLC_GO/internal/modules/video_interaction/cache"
	VideoInteractionRepositoryPackage "MLC_GO/internal/modules/video_interaction/repository"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/spf13/viper"
)

// Run with MLC_INTERACTION_REPROJECT_INTEGRATION=1 to verify bounded MySQL-to-Redis repair.
func TestHGInteractionReprojectorRepairsRedisFromMySQL(t *testing.T) {
	if os.Getenv("MLC_INTERACTION_REPROJECT_INTEGRATION") != "1" {
		t.Skip("set MLC_INTERACTION_REPROJECT_INTEGRATION=1 to run MySQL/Redis reproject integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dsn := os.Getenv("MLC_INTERACTION_MYSQL_DSN")
	if dsn == "" {
		dsn = "root:hh109@tcp(127.0.0.1:3306)/HG_MLC_DB?parseTime=true&loc=UTC"
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	viper.Set("redis.host", hgEnvOrDefault("MLC_INTERACTION_REDIS_HOST", "127.0.0.1"))
	viper.Set("redis.port", hgEnvOrDefault("MLC_INTERACTION_REDIS_PORT", "6379"))
	t.Cleanup(viper.Reset)
	redisService, err := PersistenceRedisPackage.NewRedisServiceWithError(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer redisService.Close()

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	userID := "reproject-user-" + suffix
	submissionID := "reproject-submission-" + suffix
	followeeID := "reproject-followee-" + suffix
	updatedAt := time.Date(1971, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := db.ExecContext(ctx, `INSERT INTO video_user_interactions
		(user_id, submission_id, interaction_type, active, quantity, updated_at) VALUES (?, ?, 'like', 1, 0, ?)`, userID, submissionID, updatedAt); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO user_follow_relations
		(follower_id, followee_id, active, updated_at) VALUES (?, ?, 1, ?)`, userID, followeeID, updatedAt); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO video_interaction_stat_shards
		(submission_id, shard_id, like_count, coin_count, favorite_count, share_count, updated_at) VALUES (?, 0, 7, 2, 3, 4, ?)`, submissionID, updatedAt); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO user_follow_stat_shards
		(user_id, shard_id, follower_count, updated_at) VALUES (?, 0, 9, ?)`, followeeID, updatedAt); err != nil {
		t.Fatal(err)
	}

	client := redisService.Client()
	for _, stream := range hgProjectionStreams {
		_ = client.Del(ctx, PersistenceRedisPackage.GetInteractionReprojectLeaseKey(string(stream)), PersistenceRedisPackage.GetInteractionReprojectCheckpointKey(string(stream))).Err()
	}
	stateKey := PersistenceRedisPackage.GetVideoInteractionStateKey(userID, submissionID)
	videoCountKey := PersistenceRedisPackage.GetVideoInteractionCountKey(submissionID)
	followStateKey := PersistenceRedisPackage.GetUserFollowStateKey(userID, followeeID)
	followCountKey := PersistenceRedisPackage.GetUserFollowCountKey(followeeID)
	t.Cleanup(func() {
		_ = client.Del(context.Background(), stateKey, videoCountKey, followStateKey, followCountKey).Err()
	})
	_ = client.HSet(ctx, stateKey, "like", 0).Err()
	_ = client.HSet(ctx, videoCountKey, "like", 99, "coin", 99, "favorite", 99, "share", 99).Err()
	_ = client.HSet(ctx, followStateKey, "follow", 0).Err()
	_ = client.HSet(ctx, followCountKey, "follow", 99).Err()

	reprojector, err := NewHGReprojector(
		VideoInteractionRepositoryPackage.NewRepository(db),
		VideoInteractionCachePackage.NewCache(redisService),
		HGReprojectConfig{Interval: time.Minute, Timeout: 20 * time.Second, SafetyLag: time.Second, LeaseTTL: 30 * time.Second, PageSize: HGMaxProjectionPageSize},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := reprojector.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	values, err := client.HMGet(ctx, stateKey, "like").Result()
	if err != nil || values[0] != "1" {
		t.Fatalf("video state = %#v err=%v", values, err)
	}
	values, err = client.HMGet(ctx, videoCountKey, "like", "coin", "favorite", "share").Result()
	if err != nil || fmt.Sprint(values) != "[7 2 3 4]" {
		t.Fatalf("video counts = %#v err=%v", values, err)
	}
	values, err = client.HMGet(ctx, followStateKey, "follow").Result()
	if err != nil || values[0] != "1" {
		t.Fatalf("follow state = %#v err=%v", values, err)
	}
	values, err = client.HMGet(ctx, followCountKey, "follow").Result()
	if err != nil || values[0] != "9" {
		t.Fatalf("follow count = %#v err=%v", values, err)
	}
}

func hgEnvOrDefault(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
