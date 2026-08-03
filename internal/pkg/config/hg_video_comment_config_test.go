package ConfigPackage

import (
	"testing"

	"github.com/spf13/viper"
)

func TestGetVideoCommentConfigRequiresS3SecretsFromEnvironment(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("video_comment.storage.type", "s3")
	viper.Set("video_comment.storage.endpoint", "https://s3.example.com")
	viper.Set("video_comment.storage.region", "ap-southeast-1")
	viper.Set("video_comment.storage.bucket", "comment-images")
	viper.Set("video_comment.storage.cdn_base_url", "https://cdn.example.com")
	viper.Set("video_comment.storage.request_timeout", "10s")
	viper.Set("video_comment.image.user_capacity_bytes", int64(100<<20))
	viper.Set("video_comment.image.rate_user_capacity", 6)
	viper.Set("video_comment.image.rate_ip_capacity", 30)
	viper.Set("video_comment.image.rate_window", "1m")
	viper.Set("video_comment.maintenance.enabled", true)
	viper.Set("video_comment.maintenance.interval", "1m")
	viper.Set("video_comment.maintenance.timeout", "20s")
	viper.Set("video_comment.maintenance.orphan_age", "24h")
	viper.Set("video_comment.maintenance.batch_size", 100)

	if _, err := GetVideoCommentConfig(); err == nil {
		t.Fatal("GetVideoCommentConfig() expected missing S3 credential error")
	}
	t.Setenv("VIDEO_COMMENT_S3_ACCESS_KEY_ID", "access")
	t.Setenv("VIDEO_COMMENT_S3_SECRET_ACCESS_KEY", "secret")
	cfg, err := GetVideoCommentConfig()
	if err != nil {
		t.Fatalf("GetVideoCommentConfig() error=%v", err)
	}
	if cfg.Storage.CDNBaseURL != "https://cdn.example.com" || cfg.Image.UserCapacityBytes != 100<<20 {
		t.Fatalf("GetVideoCommentConfig() cfg=%+v", cfg)
	}
}
