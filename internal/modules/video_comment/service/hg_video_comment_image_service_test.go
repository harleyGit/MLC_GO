package VideoCommentServicePackage

import (
	VideoCommentRepositoryPackage "MLC_GO/internal/modules/video_comment/repository"
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

type hgFakeCommentImageUploader struct {
	size  int64
	ext   string
	url   string
	key   string
	err   error
	calls *[]string
}

func (f *hgFakeCommentImageUploader) Prepare(ext string) (HGImageUpload, error) {
	f.ext = ext
	if f.key == "" {
		f.key = "video_comment/a.png"
	}
	if f.calls != nil {
		*f.calls = append(*f.calls, "prepare")
	}
	return HGImageUpload{URL: f.url, StorageKey: f.key}, nil
}

func (f *hgFakeCommentImageUploader) UploadFromReader(_ context.Context, _ io.Reader, size int64, ext, key string) error {
	f.size, f.ext, f.key = size, ext, key
	if f.calls != nil {
		*f.calls = append(*f.calls, "upload")
	}
	return f.err
}

func (f *hgFakeCommentImageUploader) Delete(_ context.Context, _ string) error { return nil }

type hgFakeImageGuard struct{ calls int }

func (g *hgFakeImageGuard) Allow(context.Context, string, string) error { g.calls++; return nil }

type hgFakeImageAssets struct {
	reserved bool
	cleanup  bool
	asset    VideoCommentRepositoryPackage.HGImageAsset
	calls    *[]string
}

func (a *hgFakeImageAssets) ReserveImageAsset(_ context.Context, asset VideoCommentRepositoryPackage.HGImageAsset, _ int64) error {
	a.reserved = true
	a.asset = asset
	if a.calls != nil {
		*a.calls = append(*a.calls, "reserve")
	}
	return nil
}
func (a *hgFakeImageAssets) ScheduleImageCleanup(context.Context, string, string) error {
	a.cleanup = true
	return nil
}

func TestUploadImageAcceptsOnlyFiveMiBJPEGPngAndWebP(t *testing.T) {
	uploader := &hgFakeCommentImageUploader{url: "http://localhost:8080/uploads/video_comment/a.png"}
	guard := &hgFakeImageGuard{}
	assets := &hgFakeImageAssets{}
	service := NewServiceWithImageDependencies(&hgFakeCommentRepository{}, uploader, guard, assets, 100<<20)

	result, err := service.UploadImage(context.Background(), "user-1", "203.0.113.8", strings.NewReader("png"), 3, "PNG")
	if err != nil || result.ImageURL != uploader.url || uploader.ext != "png" {
		t.Fatalf("UploadImage() result=%+v error=%v ext=%q", result, err, uploader.ext)
	}
	if guard.calls != 1 || !assets.reserved || assets.asset.StorageKey != uploader.key {
		t.Fatalf("UploadImage() guard=%d reserved=%v asset=%+v", guard.calls, assets.reserved, assets.asset)
	}
	for _, tc := range []struct {
		size int64
		ext  string
	}{
		{size: 0, ext: "png"},
		{size: (5 << 20) + 1, ext: "png"},
		{size: 3, ext: "gif"},
	} {
		_, err := service.UploadImage(context.Background(), "user-1", "203.0.113.8", strings.NewReader("png"), tc.size, tc.ext)
		if !errors.Is(err, ErrInvalidImageUpload) {
			t.Fatalf("UploadImage(size=%d, ext=%q) error=%v, want ErrInvalidImageUpload", tc.size, tc.ext, err)
		}
	}
}

func TestUploadImagePersistsReservationBeforeObjectUpload(t *testing.T) {
	calls := make([]string, 0, 3)
	uploader := &hgFakeCommentImageUploader{url: "https://cdn.example.com/video_comment/a.png", calls: &calls}
	assets := &hgFakeImageAssets{calls: &calls}
	service := NewServiceWithImageDependencies(&hgFakeCommentRepository{}, uploader, &hgFakeImageGuard{}, assets, 100<<20)

	_, err := service.UploadImage(context.Background(), "user-1", "203.0.113.8", strings.NewReader("png"), 3, "png")
	if err != nil {
		t.Fatalf("UploadImage() error=%v", err)
	}
	if strings.Join(calls, ",") != "prepare,reserve,upload" {
		t.Fatalf("UploadImage() calls=%v, want prepare,reserve,upload", calls)
	}
}

func TestUploadImageSchedulesDurableCleanupWhenObjectUploadFails(t *testing.T) {
	uploader := &hgFakeCommentImageUploader{url: "https://cdn.example.com/video_comment/a.png", err: errors.New("put failed")}
	assets := &hgFakeImageAssets{}
	service := NewServiceWithImageDependencies(&hgFakeCommentRepository{}, uploader, &hgFakeImageGuard{}, assets, 100<<20)

	_, err := service.UploadImage(context.Background(), "user-1", "203.0.113.8", strings.NewReader("png"), 3, "png")
	if err == nil || !assets.cleanup {
		t.Fatalf("UploadImage() error=%v cleanup=%v", err, assets.cleanup)
	}
}

func TestDetectCommentImageExtSupportsOptionalQueryExt(t *testing.T) {
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	ext, reader, err := DetectCommentImageExt(bytes.NewReader(png))
	if err != nil || ext != "png" {
		t.Fatalf("DetectCommentImageExt() ext=%q error=%v", ext, err)
	}
	data, err := io.ReadAll(reader)
	if err != nil || !bytes.Equal(data, png) {
		t.Fatalf("DetectCommentImageExt() restored data=%x error=%v", data, err)
	}
}
