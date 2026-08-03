package HGUploadPackage

import (
	"bytes"
	"context"
	"fmt"
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
	if err := hgWaitForCDNObject(ctx, http.DefaultClient, url, body, 500*time.Millisecond); err != nil {
		t.Fatalf("CDN GET probe error=%v", err)
	}
	if err := driver.DeleteContext(ctx, key); err != nil {
		t.Fatalf("DELETE probe error=%v", err)
	}
	// 第二次 DELETE 验证 S3 幂等重试契约；不使用 HEAD，避免为发布凭据额外要求 ListBucket 权限。
	if err := driver.DeleteContext(ctx, key); err != nil {
		t.Fatalf("second DELETE probe error=%v", err)
	}
}

// hgWaitForCDNObject 在总 context 内等待 CDN 传播，并严格比较完整探针内容，避免仅凭 200 将旧缓存或错误对象判为成功。
func hgWaitForCDNObject(ctx context.Context, client *http.Client, url string, expected []byte, retryInterval time.Duration) error {
	var lastStatus int
	var lastBody []byte
	for {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		response, err := client.Do(request)
		if err == nil {
			lastStatus = response.StatusCode
			lastBody, err = io.ReadAll(io.LimitReader(response.Body, 4096))
			response.Body.Close()
			if err == nil && response.StatusCode == http.StatusOK && bytes.Equal(lastBody, expected) {
				return nil
			}
		}
		timer := time.NewTimer(retryInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("CDN object not visible before timeout: status=%d body=%q: %w", lastStatus, lastBody, ctx.Err())
		case <-timer.C:
		}
	}
}
