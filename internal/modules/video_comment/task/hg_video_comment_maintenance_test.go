package VideoCommentTaskPackage

import (
	VideoCommentRepositoryPackage "MLC_GO/internal/modules/video_comment/repository"
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type hgFakeMaintenanceRepo struct {
	projected      int
	casMisses      int
	assets         []VideoCommentRepositoryPackage.HGImageCleanupAsset
	completed      []string
	projectErr     error
	claims         int
	dirtyOldest    time.Time
	cleanupOldest  time.Time
	leaseReclaims  int
	maintenanceErr error
	releaseErr     error
}

func (r *hgFakeMaintenanceRepo) ProjectReactionCounts(context.Context, int) (VideoCommentRepositoryPackage.HGReactionProjectionResult, error) {
	return VideoCommentRepositoryPackage.HGReactionProjectionResult{Projected: r.projected, CASMisses: r.casMisses}, r.projectErr
}
func (r *hgFakeMaintenanceRepo) ProjectReplyCounts(context.Context, int) (VideoCommentRepositoryPackage.HGReplyProjectionResult, error) {
	return VideoCommentRepositoryPackage.HGReplyProjectionResult{}, nil
}
func (r *hgFakeMaintenanceRepo) ClaimImageCleanup(context.Context, time.Time, int, time.Duration) (VideoCommentRepositoryPackage.HGImageCleanupClaim, error) {
	r.claims++
	return VideoCommentRepositoryPackage.HGImageCleanupClaim{Assets: r.assets, ExpiredLeaseReclaims: r.leaseReclaims}, nil
}
func (r *hgFakeMaintenanceRepo) CompleteImageCleanup(_ context.Context, asset VideoCommentRepositoryPackage.HGImageCleanupAsset) error {
	r.completed = append(r.completed, asset.ImageID)
	return nil
}
func (r *hgFakeMaintenanceRepo) ReleaseImageCleanup(context.Context, VideoCommentRepositoryPackage.HGImageCleanupAsset) error {
	return r.releaseErr
}
func (r *hgFakeMaintenanceRepo) MaintenanceOldestTimes(context.Context, time.Time, time.Duration) (time.Time, time.Time, error) {
	return r.dirtyOldest, r.cleanupOldest, r.maintenanceErr
}

type hgFakeMaintenanceStorage struct {
	deleted   []string
	deleteErr error
}

func TestHGVideoCommentMaintenanceCountsAndReturnsCleanupReleaseFailure(t *testing.T) {
	hgResetVideoCommentMaintenanceMetricsForTest()
	t.Cleanup(hgResetVideoCommentMaintenanceMetricsForTest)
	repo := &hgFakeMaintenanceRepo{
		assets:     []VideoCommentRepositoryPackage.HGImageCleanupAsset{{ImageID: "CIMG_1", StorageKey: "video_comment/a.png", CleanupToken: "token-1"}},
		releaseErr: errors.New("mysql unavailable"),
	}
	worker, err := NewHGVideoCommentMaintenance(repo, &hgFakeMaintenanceStorage{deleteErr: errors.New("s3 unavailable")}, HGVideoCommentMaintenanceConfig{Interval: time.Minute, Timeout: 10 * time.Second, OrphanAge: time.Hour, BatchSize: 100})
	if err != nil {
		t.Fatalf("NewHGVideoCommentMaintenance() error=%v", err)
	}
	err = worker.RunOnce(context.Background())
	if err == nil || !strings.Contains(err.Error(), "release image CIMG_1") {
		t.Fatalf("RunOnce() error=%v", err)
	}
	if hgImageCleanupFailures.Load() != 2 {
		t.Fatalf("cleanup failures=%d, want 2", hgImageCleanupFailures.Load())
	}
}

func (s *hgFakeMaintenanceStorage) Delete(_ context.Context, key string) error {
	s.deleted = append(s.deleted, key)
	return s.deleteErr
}

func TestHGVideoCommentMaintenanceExportsBacklogAndFailureMetrics(t *testing.T) {
	hgResetVideoCommentMaintenanceMetricsForTest()
	t.Cleanup(hgResetVideoCommentMaintenanceMetricsForTest)
	now := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	repo := &hgFakeMaintenanceRepo{
		projected: 1, casMisses: 2, dirtyOldest: now.Add(-5 * time.Minute), cleanupOldest: now.Add(-9 * time.Minute), leaseReclaims: 3,
		assets: []VideoCommentRepositoryPackage.HGImageCleanupAsset{{ImageID: "CIMG_1", StorageKey: "video_comment/a.png", CleanupToken: "token-1"}},
	}
	storage := &hgFakeMaintenanceStorage{deleteErr: errors.New("delete denied")}
	worker, err := NewHGVideoCommentMaintenance(repo, storage, HGVideoCommentMaintenanceConfig{Interval: time.Minute, Timeout: 10 * time.Second, OrphanAge: time.Hour, BatchSize: 100})
	if err != nil {
		t.Fatalf("NewHGVideoCommentMaintenance() error=%v", err)
	}
	worker.now = func() time.Time { return now }
	if err := worker.RunOnce(context.Background()); err == nil {
		t.Fatal("RunOnce() expected cleanup error")
	}

	var output bytes.Buffer
	HGWritePrometheusMetrics(&output)
	metrics := output.String()
	for _, expected := range []string{
		"mlc_video_comment_reaction_dirty_oldest_age_seconds 300",
		"mlc_video_comment_image_cleanup_oldest_age_seconds 540",
		"mlc_video_comment_reaction_projection_cas_misses_total 2",
		"mlc_video_comment_image_cleanup_expired_lease_reclaims_total 3",
		"mlc_video_comment_image_cleanup_failures_total 1",
	} {
		if !strings.Contains(metrics, expected) {
			t.Fatalf("metrics missing %q: %s", expected, metrics)
		}
	}
}

func TestHGVideoCommentMaintenanceClearsAgeGaugesWhenBacklogIsEmpty(t *testing.T) {
	hgResetVideoCommentMaintenanceMetricsForTest()
	t.Cleanup(hgResetVideoCommentMaintenanceMetricsForTest)
	hgReactionDirtyOldestAgeSeconds.Store(20)
	hgImageCleanupOldestAgeSeconds.Store(30)
	repo := &hgFakeMaintenanceRepo{}
	worker, err := NewHGVideoCommentMaintenance(repo, &hgFakeMaintenanceStorage{}, HGVideoCommentMaintenanceConfig{Interval: time.Minute, Timeout: 10 * time.Second, OrphanAge: time.Hour, BatchSize: 100})
	if err != nil {
		t.Fatalf("NewHGVideoCommentMaintenance() error=%v", err)
	}
	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error=%v", err)
	}
	if hgReactionDirtyOldestAgeSeconds.Load() != 0 || hgImageCleanupOldestAgeSeconds.Load() != 0 {
		t.Fatalf("age gauges dirty=%d cleanup=%d", hgReactionDirtyOldestAgeSeconds.Load(), hgImageCleanupOldestAgeSeconds.Load())
	}
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
