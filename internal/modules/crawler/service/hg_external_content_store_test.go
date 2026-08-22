package service

import (
	CrawlerPlatformPackage "MLC_GO/internal/modules/crawler/platform"
	"context"
	"errors"
	"testing"
)

type hgExternalContentRepositoryStub struct {
	inserted int64
	err      error
	calls    int
	taskID   uint64
	runID    uint64
}

func (s *hgExternalContentRepositoryStub) UpsertRecommendationsWithInserted(context.Context, []CrawlerPlatformPackage.HGRecommendation) (int64, error) {
	s.calls++
	return s.inserted, s.err
}

func (s *hgExternalContentRepositoryStub) UpsertTaskRecommendationsWithInserted(_ context.Context, taskID, runID uint64, _ []CrawlerPlatformPackage.HGRecommendation) (int64, error) {
	s.calls++
	s.taskID, s.runID = taskID, runID
	return s.inserted, s.err
}

type hgExternalContentCacheStub struct {
	increments    []int64
	invalidates   int
	incrementErr  error
	invalidateErr error
}

func (s *hgExternalContentCacheStub) IncrementExternalCounterIfPresent(_ context.Context, delta int64) error {
	s.increments = append(s.increments, delta)
	return s.incrementErr
}

func (s *hgExternalContentCacheStub) InvalidateVideoListPages(context.Context) error {
	s.invalidates++
	return s.invalidateErr
}

func TestHGExternalContentStoreUpdatesCacheAfterCommit(t *testing.T) {
	repository := &hgExternalContentRepositoryStub{inserted: 2}
	cache := &hgExternalContentCacheStub{incrementErr: errors.New("redis increment failed"), invalidateErr: errors.New("redis invalidation failed")}
	store := NewHGExternalContentStore(repository, cache)

	err := store.UpsertRecommendations(context.Background(), []CrawlerPlatformPackage.HGRecommendation{{Platform: "bilibili", ContentID: "BV1"}})
	if err != nil {
		t.Fatalf("UpsertRecommendations() error = %v", err)
	}
	if repository.calls != 1 || len(cache.increments) != 1 || cache.increments[0] != 2 || cache.invalidates != 1 {
		t.Fatalf("calls: repository=%d increments=%v invalidates=%d", repository.calls, cache.increments, cache.invalidates)
	}
}

func TestHGExternalContentStoreInvalidatesUpdatedBatchWithoutIncrement(t *testing.T) {
	repository := &hgExternalContentRepositoryStub{}
	cache := &hgExternalContentCacheStub{}
	store := NewHGExternalContentStore(repository, cache)

	err := store.UpsertRecommendations(context.Background(), []CrawlerPlatformPackage.HGRecommendation{{Platform: "bilibili", ContentID: "BV1"}})
	if err != nil {
		t.Fatalf("UpsertRecommendations() error = %v", err)
	}
	if len(cache.increments) != 0 || cache.invalidates != 1 {
		t.Fatalf("increments=%v invalidates=%d", cache.increments, cache.invalidates)
	}
}

func TestHGExternalContentStoreSkipsCacheWhenDatabaseFails(t *testing.T) {
	databaseErr := errors.New("database failed")
	repository := &hgExternalContentRepositoryStub{err: databaseErr}
	cache := &hgExternalContentCacheStub{}
	store := NewHGExternalContentStore(repository, cache)

	err := store.UpsertRecommendations(context.Background(), []CrawlerPlatformPackage.HGRecommendation{{Platform: "bilibili", ContentID: "BV1"}})
	if !errors.Is(err, databaseErr) {
		t.Fatalf("UpsertRecommendations() error = %v", err)
	}
	if len(cache.increments) != 0 || cache.invalidates != 0 {
		t.Fatalf("cache called before commit: increments=%v invalidates=%d", cache.increments, cache.invalidates)
	}
}

func TestHGExternalContentStoreSkipsEmptyBatch(t *testing.T) {
	repository := &hgExternalContentRepositoryStub{}
	cache := &hgExternalContentCacheStub{}
	store := NewHGExternalContentStore(repository, cache)

	if err := store.UpsertRecommendations(context.Background(), nil); err != nil {
		t.Fatalf("UpsertRecommendations() error = %v", err)
	}
	if repository.calls != 0 || cache.invalidates != 0 {
		t.Fatalf("empty batch calls: repository=%d invalidates=%d", repository.calls, cache.invalidates)
	}
}

func TestHGExternalContentStoreTaskWriteUpdatesCacheAfterCommit(t *testing.T) {
	repository := &hgExternalContentRepositoryStub{inserted: 1}
	cache := &hgExternalContentCacheStub{}
	store := NewHGExternalContentStore(repository, cache)

	if err := store.UpsertTaskRecommendations(context.Background(), 9, 11, []CrawlerPlatformPackage.HGRecommendation{{Platform: "bilibili", ContentID: "BV1"}}); err != nil {
		t.Fatal(err)
	}
	if repository.taskID != 9 || repository.runID != 11 || len(cache.increments) != 1 || cache.invalidates != 1 {
		t.Fatalf("repository task=%d run=%d cache=%#v", repository.taskID, repository.runID, cache)
	}
}
