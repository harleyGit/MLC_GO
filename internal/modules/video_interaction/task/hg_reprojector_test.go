package VideoInteractionTaskPackage

import (
	VideoInteractionRepositoryPackage "MLC_GO/internal/modules/video_interaction/repository"
	"context"
	"errors"
	"testing"
	"time"
)

func TestHGReprojectorRunsFourBoundedStreamsWithSafetyLag(t *testing.T) {
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	repo := &hgFakeProjectionRepository{}
	cache := &hgFakeProjectionCache{checkpoints: make(map[HGProjectionStream]string)}
	reprojector, err := NewHGReprojector(repo, cache, HGReprojectConfig{
		Interval: time.Minute, Timeout: 10 * time.Second, SafetyLag: 5 * time.Second,
		LeaseTTL: 15 * time.Second, PageSize: HGMaxProjectionPageSize + 50,
	})
	if err != nil {
		t.Fatalf("NewHGReprojector() error = %v", err)
	}
	reprojector.hgNow = func() time.Time { return now }

	if err := reprojector.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if len(repo.calls) != 4 {
		t.Fatalf("repository calls = %d, want 4", len(repo.calls))
	}
	for _, call := range repo.calls {
		if call.limit != HGMaxProjectionPageSize {
			t.Fatalf("stream %s limit = %d, want hard cap %d", call.stream, call.limit, HGMaxProjectionPageSize)
		}
		if !call.cutoff.Equal(now.Add(-5 * time.Second)) {
			t.Fatalf("stream %s cutoff = %v", call.stream, call.cutoff)
		}
	}
}

func TestHGReprojectorCommitsNextCheckpointWithLeaseToken(t *testing.T) {
	cursor := VideoInteractionRepositoryPackage.HGProjectionCursor{UpdatedAt: time.Unix(100, 0).UTC(), RowID: 9}
	repo := &hgFakeProjectionRepository{videoStates: []VideoInteractionRepositoryPackage.HGVideoStateProjection{{
		UserID: "user-1", SubmissionID: "submission-1", InteractionType: "like", Active: true, Cursor: cursor,
	}}}
	cache := &hgFakeProjectionCache{checkpoints: make(map[HGProjectionStream]string)}
	reprojector, err := NewHGReprojector(repo, cache, HGReprojectConfig{
		Interval: time.Minute, Timeout: 10 * time.Second, SafetyLag: time.Second, LeaseTTL: 15 * time.Second, PageSize: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := reprojector.hgRunStream(context.Background(), HGProjectionStreamVideoState, time.Now()); err != nil {
		t.Fatalf("hgRunStream() error = %v", err)
	}
	if cache.committedToken != "token-video_state" {
		t.Fatalf("commit token = %q", cache.committedToken)
	}
	if cache.committedCheckpoint == "" {
		t.Fatal("expected non-empty checkpoint for a full page")
	}
	if len(cache.videoStates) != 1 || !cache.videoStates[0].Active {
		t.Fatalf("applied video states = %#v", cache.videoStates)
	}
}

func TestHGReprojectorKeepsLastCursorForPartialPage(t *testing.T) {
	cursor := VideoInteractionRepositoryPackage.HGProjectionCursor{UpdatedAt: time.Unix(101, 0).UTC(), RowID: 10}
	repo := &hgFakeProjectionRepository{videoStates: []VideoInteractionRepositoryPackage.HGVideoStateProjection{{
		UserID: "user-1", SubmissionID: "submission-1", InteractionType: "like", Active: true, Cursor: cursor,
	}}}
	cache := &hgFakeProjectionCache{checkpoints: make(map[HGProjectionStream]string)}
	reprojector, err := NewHGReprojector(repo, cache, HGReprojectConfig{
		Interval: time.Minute, Timeout: 10 * time.Second, SafetyLag: time.Second, LeaseTTL: 15 * time.Second, PageSize: 10,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := reprojector.hgRunStream(context.Background(), HGProjectionStreamVideoState, time.Now()); err != nil {
		t.Fatalf("hgRunStream() error = %v", err)
	}
	if cache.committedCheckpoint == "" {
		t.Fatal("partial page must retain the last cursor instead of restarting from the beginning")
	}
}

func TestHGReprojectorDoesNotReadOrWriteWithoutLease(t *testing.T) {
	repo := &hgFakeProjectionRepository{}
	cache := &hgFakeProjectionCache{leaseHeld: true, checkpoints: make(map[HGProjectionStream]string)}
	reprojector, err := NewHGReprojector(repo, cache, HGReprojectConfig{
		Interval: time.Minute, Timeout: 10 * time.Second, SafetyLag: time.Second, LeaseTTL: 15 * time.Second, PageSize: 10,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := reprojector.hgRunStream(context.Background(), HGProjectionStreamVideoState, time.Now()); err != nil {
		t.Fatalf("hgRunStream() error = %v", err)
	}
	if len(repo.calls) != 0 || cache.committedToken != "" {
		t.Fatal("a lease loser must not query MySQL or commit a checkpoint")
	}
}

func TestHGReprojectorReleasesLeaseAfterProjectionFailure(t *testing.T) {
	repo := &hgFakeProjectionRepository{err: errors.New("mysql unavailable")}
	cache := &hgFakeProjectionCache{checkpoints: make(map[HGProjectionStream]string)}
	reprojector, err := NewHGReprojector(repo, cache, HGReprojectConfig{
		Interval: time.Minute, Timeout: 10 * time.Second, SafetyLag: time.Second, LeaseTTL: 15 * time.Second, PageSize: 10,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := reprojector.hgRunStream(context.Background(), HGProjectionStreamVideoState, time.Now()); err == nil {
		t.Fatal("expected repository failure")
	}
	if cache.releasedToken != "token-video_state" {
		t.Fatalf("released token = %q", cache.releasedToken)
	}
}

type hgProjectionCall struct {
	stream HGProjectionStream
	cutoff time.Time
	limit  int
}

type hgFakeProjectionRepository struct {
	calls       []hgProjectionCall
	videoStates []VideoInteractionRepositoryPackage.HGVideoStateProjection
	err         error
}

func (r *hgFakeProjectionRepository) ListVideoStates(_ context.Context, _ VideoInteractionRepositoryPackage.HGProjectionCursor, cutoff time.Time, limit int) ([]VideoInteractionRepositoryPackage.HGVideoStateProjection, error) {
	r.calls = append(r.calls, hgProjectionCall{HGProjectionStreamVideoState, cutoff, limit})
	return r.videoStates, r.err
}

func (r *hgFakeProjectionRepository) ListFollowStates(_ context.Context, _ VideoInteractionRepositoryPackage.HGProjectionCursor, cutoff time.Time, limit int) ([]VideoInteractionRepositoryPackage.HGFollowStateProjection, error) {
	r.calls = append(r.calls, hgProjectionCall{HGProjectionStreamFollowState, cutoff, limit})
	return nil, r.err
}

func (r *hgFakeProjectionRepository) ListVideoCounts(_ context.Context, _ VideoInteractionRepositoryPackage.HGProjectionCursor, cutoff time.Time, limit int) ([]VideoInteractionRepositoryPackage.HGVideoCountProjection, error) {
	r.calls = append(r.calls, hgProjectionCall{HGProjectionStreamVideoCounts, cutoff, limit})
	return nil, r.err
}

func (r *hgFakeProjectionRepository) ListFollowCounts(_ context.Context, _ VideoInteractionRepositoryPackage.HGProjectionCursor, cutoff time.Time, limit int) ([]VideoInteractionRepositoryPackage.HGFollowCountProjection, error) {
	r.calls = append(r.calls, hgProjectionCall{HGProjectionStreamFollowCounts, cutoff, limit})
	return nil, r.err
}

type hgFakeProjectionCache struct {
	leaseHeld           bool
	checkpoints         map[HGProjectionStream]string
	videoStates         []VideoInteractionRepositoryPackage.HGVideoStateProjection
	committedToken      string
	committedCheckpoint string
	releasedToken       string
}

func (c *hgFakeProjectionCache) AcquireLease(_ context.Context, rawStream string, _ time.Duration) (string, bool, error) {
	stream := HGProjectionStream(rawStream)
	if c.leaseHeld {
		return "", false, nil
	}
	return "token-" + string(stream), true, nil
}

func (c *hgFakeProjectionCache) LoadCheckpoint(_ context.Context, rawStream string) (string, error) {
	stream := HGProjectionStream(rawStream)
	return c.checkpoints[stream], nil
}

func (c *hgFakeProjectionCache) ApplyVideoStates(_ context.Context, rows []VideoInteractionRepositoryPackage.HGVideoStateProjection) error {
	c.videoStates = append(c.videoStates, rows...)
	return nil
}

func (c *hgFakeProjectionCache) ApplyFollowStates(context.Context, []VideoInteractionRepositoryPackage.HGFollowStateProjection) error {
	return nil
}

func (c *hgFakeProjectionCache) ApplyVideoCounts(context.Context, []VideoInteractionRepositoryPackage.HGVideoCountProjection) error {
	return nil
}

func (c *hgFakeProjectionCache) ApplyFollowCounts(context.Context, []VideoInteractionRepositoryPackage.HGFollowCountProjection) error {
	return nil
}

func (c *hgFakeProjectionCache) CommitCheckpoint(_ context.Context, _ string, token string, checkpoint string) error {
	c.committedToken = token
	c.committedCheckpoint = checkpoint
	return nil
}

func (c *hgFakeProjectionCache) ReleaseLease(_ context.Context, _ string, token string) error {
	c.releasedToken = token
	return nil
}
