package service

import (
	CrawlerPlatformPackage "MLC_GO/internal/modules/crawler/platform"
	"context"
)

// HGExternalContentRepository persists crawler recommendations and reports rows newly inserted by the committed batch.
type HGExternalContentRepository interface {
	UpsertRecommendationsWithInserted(ctx context.Context, items []CrawlerPlatformPackage.HGRecommendation) (int64, error)
	UpsertTaskRecommendationsWithInserted(ctx context.Context, taskID, runID uint64, items []CrawlerPlatformPackage.HGRecommendation) (int64, error)
}

// HGExternalContentCache maintains the video-list read model after crawler writes.
type HGExternalContentCache interface {
	IncrementExternalCounterIfPresent(ctx context.Context, delta int64) error
	InvalidateVideoListPages(ctx context.Context) error
}

// HGExternalContentStore adapts the crawler persistence boundary to write-after cache maintenance.
// MySQL commit is authoritative; Redis counter and page invalidation failures are independent best-effort operations.
type HGExternalContentStore struct {
	repository HGExternalContentRepository
	cache      HGExternalContentCache
}

// NewHGExternalContentStore creates the crawler store adapter used by both application entry points.
func NewHGExternalContentStore(repository HGExternalContentRepository, cache HGExternalContentCache) *HGExternalContentStore {
	return &HGExternalContentStore{repository: repository, cache: cache}
}

// UpsertRecommendations commits MySQL first, then updates the initialized counter and invalidates every cached list page.
func (s *HGExternalContentStore) UpsertRecommendations(ctx context.Context, items []CrawlerPlatformPackage.HGRecommendation) error {
	if len(items) == 0 {
		return nil
	}
	inserted, err := s.repository.UpsertRecommendationsWithInserted(ctx, items)
	return s.hgUpdateCacheAfterCommit(ctx, inserted, err)
}

// UpsertTaskRecommendations atomically persists global content and associations before post-commit cache maintenance.
func (s *HGExternalContentStore) UpsertTaskRecommendations(ctx context.Context, taskID, runID uint64, items []CrawlerPlatformPackage.HGRecommendation) error {
	if len(items) == 0 {
		return nil
	}
	inserted, err := s.repository.UpsertTaskRecommendationsWithInserted(ctx, taskID, runID, items)
	return s.hgUpdateCacheAfterCommit(ctx, inserted, err)
}

func (s *HGExternalContentStore) hgUpdateCacheAfterCommit(ctx context.Context, inserted int64, err error) error {
	if err != nil {
		return err
	}
	if s.cache == nil {
		return nil
	}
	if inserted > 0 {
		_ = s.cache.IncrementExternalCounterIfPresent(ctx, inserted)
	}
	_ = s.cache.InvalidateVideoListPages(ctx)
	return nil
}
