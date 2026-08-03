package HGUploadPackage

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestS3StorageIntegrationPutCDNGetDelete(t *testing.T) {
	if os.Getenv("MLC_S3_INTEGRATION") != "1" {
		t.Skip("set MLC_S3_INTEGRATION=1 to run against a real S3-compatible bucket")
	}
	driver, err := NewS3StorageDriver(S3Config{
		Endpoint: os.Getenv("VIDEO_COMMENT_S3_ENDPOINT"), Region: os.Getenv("VIDEO_COMMENT_S3_REGION"), BucketName: os.Getenv("VIDEO_COMMENT_S3_BUCKET"),
		AccessKeyID: os.Getenv("VIDEO_COMMENT_S3_ACCESS_KEY_ID"), SecretAccessKey: os.Getenv("VIDEO_COMMENT_S3_SECRET_ACCESS_KEY"), CDNBaseURL: os.Getenv("VIDEO_COMMENT_CDN_BASE_URL"), RequestTimeout: 15 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewS3StorageDriver() error=%v", err)
	}
	body := []byte("mlc-video-comment-s3-probe")
	key := "video_comment/probe/" + strings.ReplaceAll(time.Now().UTC().Format("20060102T150405.000000000"), ".", "") + ".txt"
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	url, err := driver.UploadStreamContext(ctx, bytes.NewReader(body), key, "text/plain")
	if err != nil {
		t.Fatalf("PUT probe error=%v", err)
	}
	defer func() { _ = driver.DeleteContext(context.Background(), key) }()
	resp, err := http.DefaultClient.Get(url)
	if err != nil {
		t.Fatalf("CDN GET probe error=%v", err)
	}
	data, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
	resp.Body.Close()
	if readErr != nil || resp.StatusCode != http.StatusOK || !bytes.Equal(data, body) {
		t.Fatalf("CDN GET status=%d body=%q error=%v", resp.StatusCode, data, readErr)
	}
	if err := driver.DeleteContext(ctx, key); err != nil {
		t.Fatalf("DELETE probe error=%v", err)
	}
}
