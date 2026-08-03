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
	size int64
	ext  string
	url  string
}

func (f *hgFakeCommentImageUploader) UploadFromReader(_ context.Context, _ io.Reader, size int64, ext string) (HGImageUpload, error) {
	f.size, f.ext = size, ext
	return HGImageUpload{URL: f.url, StorageKey: "video_comment/a.png", SizeBytes: size}, nil
}

func (f *hgFakeCommentImageUploader) Delete(_ context.Context, _ string) error { return nil }

type hgFakeImageGuard struct{ calls int }

func (g *hgFakeImageGuard) Allow(context.Context, string, string) error { g.calls++; return nil }

type hgFakeImageAssets struct {
	reserved bool
	created  bool
}

func (a *hgFakeImageAssets) ReserveImageQuota(context.Context, string, int64, int64) error {
	a.reserved = true
	return nil
}
func (a *hgFakeImageAssets) ReleaseImageQuota(context.Context, string, int64) error { return nil }
func (a *hgFakeImageAssets) CreateImageAsset(context.Context, VideoCommentRepositoryPackage.HGImageAsset) error {
	a.created = true
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
	if guard.calls != 1 || !assets.reserved || !assets.created {
		t.Fatalf("UploadImage() guard=%d reserved=%v created=%v", guard.calls, assets.reserved, assets.created)
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
