package HGUploadPackage

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestS3StorageDriverUploadsWithSignatureAndReturnsCDNURL(t *testing.T) {
	var method, authorization, contentType, body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		authorization = r.Header.Get("Authorization")
		contentType = r.Header.Get("Content-Type")
		data, _ := io.ReadAll(r.Body)
		body = string(data)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	driver, err := NewS3StorageDriver(S3Config{
		Endpoint: server.URL, Region: "us-east-1", BucketName: "comments",
		AccessKeyID: "access", SecretAccessKey: "secret", CDNBaseURL: "https://cdn.example.com/base",
		RequestTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("NewS3StorageDriver() error=%v", err)
	}
	url, err := driver.UploadStreamContext(context.Background(), strings.NewReader("png"), "video_comment/a.png", "image/png")
	if err != nil {
		t.Fatalf("UploadStream() error=%v", err)
	}
	if method != http.MethodPut || !strings.HasPrefix(authorization, "AWS4-HMAC-SHA256 ") || contentType != "image/png" || body != "png" {
		t.Fatalf("request method=%q auth=%q contentType=%q body=%q", method, authorization, contentType, body)
	}
	if url != "https://cdn.example.com/base/video_comment/a.png" {
		t.Fatalf("UploadStream() url=%q", url)
	}
}
