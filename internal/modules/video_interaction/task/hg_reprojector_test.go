package VideoInteractionTaskPackage

import (
	VideoInteractionRepositoryPackage "MLC_GO/internal/modules/video_interaction/repository"
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestHGReprojectorRunsFourBoundedStreamsWithSafetyLag(t *testing.T) {
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	repo := &hgFakeProjectionRepository{}
	cache := &hgFakeProjectionCache{checkpoints: make(map[string]string)}
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
		if call.hashRange.Start != 0 || call.hashRange.End != HGProjectionHashBucketCount {
			t.Fatalf("stream %s hash range = %+v", call.stream, call.hashRange)
		}
	}
}

func TestHGReprojectorUsesFixedShardKeysAndBoundedWorkers(t *testing.T) {
	repo := &hgFakeProjectionRepository{block: make(chan struct{}), entered: make(chan struct{}, 8)}
	cache := &hgFakeProjectionCache{checkpoints: make(map[string]string)}
	reprojector, err := NewHGReprojector(repo, cache, HGReprojectConfig{
		Interval: time.Minute, Timeout: 10 * time.Second, SafetyLag: time.Second, LeaseTTL: 15 * time.Second, PageSize: 10,
		WorkerCount: 2, HashRanges: []HGProjectionHashRange{{Start: 0, End: 512}, {Start: 512, End: 1024}},
	})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- reprojector.RunOnce(context.Background()) }()
	for i := 0; i < 2; i++ {
		select {
		case <-repo.entered:
		case <-time.After(time.Second):
			t.Fatal("workers did not start")
		}
	}
	select {
	case <-repo.entered:
		t.Fatal("worker count exceeded configured bound")
	case <-time.After(20 * time.Millisecond):
	}
	close(repo.block)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if repo.maxActive.Load() > 2 {
		t.Fatalf("max active workers = %d", repo.maxActive.Load())
	}
	if !cache.acquiredKeys["video_state:0000-0512"] || !cache.acquiredKeys["video_state:0512-1024"] {
		t.Fatalf("acquired keys = %#v", cache.acquiredKeys)
	}
}

func TestHGReprojectorStartRunsImmediately(t *testing.T) {
	repo := &hgFakeProjectionRepository{called: make(chan struct{}, 1)}
	cache := &hgFakeProjectionCache{checkpoints: make(map[string]string)}
	reprojector, err := NewHGReprojector(repo, cache, HGReprojectConfig{
		Interval: time.Hour, Timeout: time.Second, SafetyLag: time.Second, LeaseTTL: 2 * time.Second, PageSize: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	reprojector.Start(context.Background())
	defer reprojector.Close()
	select {
	case <-repo.called:
	case <-time.After(time.Second):
		t.Fatal("expected an immediate startup run")
	}
}

func TestHGReprojectorCommitsNextCheckpointWithLeaseToken(t *testing.T) {
	cursor := VideoInteractionRepositoryPackage.HGProjectionCursor{UpdatedAt: time.Unix(100, 0).UTC(), RowID: 9}
	repo := &hgFakeProjectionRepository{videoStates: []VideoInteractionRepositoryPackage.HGVideoStateProjection{{
		UserID: "user-1", SubmissionID: "submission-1", InteractionType: "like", Active: true, Cursor: cursor,
	}}}
	cache := &hgFakeProjectionCache{checkpoints: make(map[string]string)}
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
	cache := &hgFakeProjectionCache{checkpoints: make(map[string]string)}
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
	cache := &hgFakeProjectionCache{leaseHeld: true, checkpoints: make(map[string]string)}
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
	cache := &hgFakeProjectionCache{checkpoints: make(map[string]string)}
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
	stream    HGProjectionStream
	cutoff    time.Time
	limit     int
	hashRange HGProjectionHashRange
}

type hgFakeProjectionRepository struct {
	mu          sync.Mutex
	calls       []hgProjectionCall
	videoStates []VideoInteractionRepositoryPackage.HGVideoStateProjection
	err         error
	block       chan struct{}
	entered     chan struct{}
	called      chan struct{}
	active      atomic.Int32
	maxActive   atomic.Int32
}

func (r *hgFakeProjectionRepository) record(stream HGProjectionStream, cutoff time.Time, limit int, hashRange HGProjectionHashRange) {
	r.mu.Lock()
	r.calls = append(r.calls, hgProjectionCall{stream: stream, cutoff: cutoff, limit: limit, hashRange: hashRange})
	r.mu.Unlock()
	if r.called != nil {
		select {
		case r.called <- struct{}{}:
		default:
		}
	}
	active := r.active.Add(1)
	for active > r.maxActive.Load() && !r.maxActive.CompareAndSwap(r.maxActive.Load(), active) {
	}
	if r.entered != nil {
		r.entered <- struct{}{}
	}
	if r.block != nil {
		<-r.block
	}
	r.active.Add(-1)
}

func (r *hgFakeProjectionRepository) ListVideoStates(_ context.Context, _ VideoInteractionRepositoryPackage.HGProjectionCursor, cutoff time.Time, limit int, hashRange HGProjectionHashRange) ([]VideoInteractionRepositoryPackage.HGVideoStateProjection, error) {
	r.record(HGProjectionStreamVideoState, cutoff, limit, hashRange)
	return r.videoStates, r.err
}

func (r *hgFakeProjectionRepository) ListFollowStates(_ context.Context, _ VideoInteractionRepositoryPackage.HGProjectionCursor, cutoff time.Time, limit int, hashRange HGProjectionHashRange) ([]VideoInteractionRepositoryPackage.HGFollowStateProjection, error) {
	r.record(HGProjectionStreamFollowState, cutoff, limit, hashRange)
	return nil, r.err
}

func (r *hgFakeProjectionRepository) ListVideoCounts(_ context.Context, _ VideoInteractionRepositoryPackage.HGProjectionCursor, cutoff time.Time, limit int, hashRange HGProjectionHashRange) ([]VideoInteractionRepositoryPackage.HGVideoCountProjection, error) {
	r.record(HGProjectionStreamVideoCounts, cutoff, limit, hashRange)
	return nil, r.err
}

func (r *hgFakeProjectionRepository) ListFollowCounts(_ context.Context, _ VideoInteractionRepositoryPackage.HGProjectionCursor, cutoff time.Time, limit int, hashRange HGProjectionHashRange) ([]VideoInteractionRepositoryPackage.HGFollowCountProjection, error) {
	r.record(HGProjectionStreamFollowCounts, cutoff, limit, hashRange)
	return nil, r.err
}

type hgFakeProjectionCache struct {
	mu                  sync.Mutex
	leaseHeld           bool
	checkpoints         map[string]string
	acquiredKeys        map[string]bool
	videoStates         []VideoInteractionRepositoryPackage.HGVideoStateProjection
	committedToken      string
	committedCheckpoint string
	releasedToken       string
}

func (c *hgFakeProjectionCache) AcquireLease(_ context.Context, rawStream string, _ time.Duration) (string, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.acquiredKeys == nil {
		c.acquiredKeys = make(map[string]bool)
	}
	c.acquiredKeys[rawStream] = true
	if c.leaseHeld {
		return "", false, nil
	}
	return "token-" + rawStream, true, nil
}

func (c *hgFakeProjectionCache) LoadCheckpoint(_ context.Context, rawStream string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.checkpoints[rawStream], nil
}

func (c *hgFakeProjectionCache) ApplyVideoStates(_ context.Context, rows []VideoInteractionRepositoryPackage.HGVideoStateProjection) error {
	c.mu.Lock()
	defer c.mu.Unlock()
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
	c.mu.Lock()
	defer c.mu.Unlock()
	c.committedToken = token
	c.committedCheckpoint = checkpoint
	return nil
}

func (c *hgFakeProjectionCache) ReleaseLease(_ context.Context, _ string, token string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.releasedToken = token
	return nil
}
