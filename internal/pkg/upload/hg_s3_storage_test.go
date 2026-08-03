package HGUploadPackage

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHGWaitForCDNObjectRetriesUntilContentIsVisible(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts < 3 {
			http.NotFound(w, nil)
			return
		}
		_, _ = w.Write([]byte("probe"))
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := hgWaitForCDNObject(ctx, server.Client(), server.URL, []byte("probe"), time.Millisecond); err != nil {
		t.Fatalf("hgWaitForCDNObject() error=%v", err)
	}
	if attempts != 3 {
		t.Fatalf("attempts=%d, want 3", attempts)
	}
}

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

func TestNewS3StorageDriverRejectsUnsafeEndpointShape(t *testing.T) {
	for _, endpoint := range []string{"ftp://s3.example.com", "https://user:pass@s3.example.com", "https:///missing-host"} {
		_, err := NewS3StorageDriver(S3Config{Endpoint: endpoint, Region: "us-east-1", BucketName: "comments", AccessKeyID: "access", SecretAccessKey: "secret", CDNBaseURL: "https://cdn.example.com", RequestTimeout: time.Second})
		if err == nil {
			t.Fatalf("NewS3StorageDriver(%q) expected error", endpoint)
		}
	}
}

type hgTrackingStorage struct {
	uploaded string
	deleted  string
}

func (s *hgTrackingStorage) Upload([]byte, string, string) (string, error) { return "", nil }
func (s *hgTrackingStorage) UploadStream(reader io.Reader, key, _ string) (string, error) {
	_, _ = io.ReadAll(reader)
	s.uploaded = key
	return "https://cdn.example.com/" + key, nil
}
func (s *hgTrackingStorage) Delete(key string) error  { s.deleted = key; return nil }
func (s *hgTrackingStorage) GetURL(key string) string { return "https://cdn.example.com/" + key }

func TestUploadFromReaderToKeyDeletesObjectWhenDeclaredSizeDiffers(t *testing.T) {
	storage := &hgTrackingStorage{}
	uploader := &Uploader{config: UploadConfig{MaxFileSize: 5 << 20, AllowedTypes: []string{"png"}}, storage: storage}
	png := append([]byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}, []byte("extra")...)
	err := uploader.UploadFromReaderToKeyContext(context.Background(), bytes.NewReader(png), int64(len(png)-1), "video_comment/a.png", "png")
	if err == nil || storage.deleted != "video_comment/a.png" {
		t.Fatalf("UploadFromReaderToKeyContext() error=%v deleted=%q", err, storage.deleted)
	}
}
