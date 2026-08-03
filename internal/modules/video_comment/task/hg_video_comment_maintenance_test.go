package VideoCommentTaskPackage

import (
	VideoCommentRepositoryPackage "MLC_GO/internal/modules/video_comment/repository"
	"context"
	"testing"
	"time"
)

type hgFakeMaintenanceRepo struct {
	projected int
	assets    []VideoCommentRepositoryPackage.HGImageCleanupAsset
	completed []string
}

func (r *hgFakeMaintenanceRepo) ProjectReactionCounts(context.Context, int) (int, error) {
	return r.projected, nil
}
func (r *hgFakeMaintenanceRepo) ClaimImageCleanup(context.Context, time.Time, int) ([]VideoCommentRepositoryPackage.HGImageCleanupAsset, error) {
	return r.assets, nil
}
func (r *hgFakeMaintenanceRepo) CompleteImageCleanup(_ context.Context, asset VideoCommentRepositoryPackage.HGImageCleanupAsset) error {
	r.completed = append(r.completed, asset.ImageID)
	return nil
}
func (r *hgFakeMaintenanceRepo) ReleaseImageCleanup(context.Context, string) error { return nil }

type hgFakeMaintenanceStorage struct{ deleted []string }

func (s *hgFakeMaintenanceStorage) Delete(_ context.Context, key string) error {
	s.deleted = append(s.deleted, key)
	return nil
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
