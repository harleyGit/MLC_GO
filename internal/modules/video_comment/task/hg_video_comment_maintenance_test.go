package VideoCommentTaskPackage

import (
	VideoCommentRepositoryPackage "MLC_GO/internal/modules/video_comment/repository"
	"context"
	"errors"
	"testing"
	"time"
)

type hgFakeMaintenanceRepo struct {
	projected  int
	assets     []VideoCommentRepositoryPackage.HGImageCleanupAsset
	completed  []string
	projectErr error
	claims     int
}

func (r *hgFakeMaintenanceRepo) ProjectReactionCounts(context.Context, int) (int, error) {
	return r.projected, r.projectErr
}
func (r *hgFakeMaintenanceRepo) ClaimImageCleanup(context.Context, time.Time, int, time.Duration) ([]VideoCommentRepositoryPackage.HGImageCleanupAsset, error) {
	r.claims++
	return r.assets, nil
}
func (r *hgFakeMaintenanceRepo) CompleteImageCleanup(_ context.Context, asset VideoCommentRepositoryPackage.HGImageCleanupAsset) error {
	r.completed = append(r.completed, asset.ImageID)
	return nil
}
func (r *hgFakeMaintenanceRepo) ReleaseImageCleanup(context.Context, VideoCommentRepositoryPackage.HGImageCleanupAsset) error {
	return nil
}

type hgFakeMaintenanceStorage struct{ deleted []string }

func (s *hgFakeMaintenanceStorage) Delete(_ context.Context, key string) error {
	s.deleted = append(s.deleted, key)
	return nil
}

func TestHGVideoCommentMaintenanceCleansImagesWhenProjectionFails(t *testing.T) {
	repo := &hgFakeMaintenanceRepo{projectErr: errors.New("projection failed"), assets: []VideoCommentRepositoryPackage.HGImageCleanupAsset{{ImageID: "CIMG_1", StorageKey: "video_comment/a.png", CleanupToken: "token-1"}}}
	storage := &hgFakeMaintenanceStorage{}
	worker, err := NewHGVideoCommentMaintenance(repo, storage, HGVideoCommentMaintenanceConfig{Interval: time.Minute, Timeout: 10 * time.Second, OrphanAge: time.Hour, BatchSize: 100})
	if err != nil {
		t.Fatalf("NewHGVideoCommentMaintenance() error=%v", err)
	}
	if err := worker.RunOnce(context.Background()); err == nil {
		t.Fatal("RunOnce() expected projection error")
	}
	if repo.claims != 1 || len(storage.deleted) != 1 {
		t.Fatalf("RunOnce() claims=%d deleted=%v", repo.claims, storage.deleted)
	}
}

func TestHGVideoCommentMaintenanceProjectsAndCleansBoundedBatch(t *testing.T) {
	repo := &hgFakeMaintenanceRepo{projected: 2, assets: []VideoCommentRepositoryPackage.HGImageCleanupAsset{{ImageID: "CIMG_1", UserID: "user-1", StorageKey: "video_comment/a.png", SizeBytes: 3}}}
	storage := &hgFakeMaintenanceStorage{}
	worker, err := NewHGVideoCommentMaintenance(repo, storage, HGVideoCommentMaintenanceConfig{Interval: time.Minute, Timeout: 10 * time.Second, OrphanAge: time.Hour, BatchSize: 100})
	if err != nil {
		t.Fatalf("NewHGVideoCommentMaintenance() error=%v", err)
	}
	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error=%v", err)
	}
	if len(storage.deleted) != 1 || storage.deleted[0] != "video_comment/a.png" || len(repo.completed) != 1 {
		t.Fatalf("deleted=%v completed=%v", storage.deleted, repo.completed)
	}
}
