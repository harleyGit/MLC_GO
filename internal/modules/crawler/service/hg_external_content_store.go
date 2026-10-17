package service

import (
	CrawlerPlatformPackage "MLC_GO/internal/modules/crawler/platform"
	"context"
)

// HGExternalContentRepository 保存推荐及任务关联，返回已提交批次实际新增的内容数。
type HGExternalContentRepository interface {
	UpsertRecommendationsWithInserted(ctx context.Context, items []CrawlerPlatformPackage.HGRecommendation) (int64, error)
	UpsertTaskRecommendationsWithInserted(ctx context.Context, taskID, runID uint64, items []CrawlerPlatformPackage.HGRecommendation) (int64, error)
}

// HGExternalContentCache 在权威写入后维护视频列表读缓存。
type HGExternalContentCache interface {
	IncrementExternalCounterIfPresent(ctx context.Context, delta int64) error
	InvalidateVideoListPages(ctx context.Context) error
}

// HGExternalContentStore 编排写后缓存维护；启用流水线时仓储同时提交详情发现Outbox。
// MySQL提交是成功边界；Redis计数与列表失效沿用独立的尽力而为策略，详情API直接读MySQL。
type HGExternalContentStore struct {
	repository HGExternalContentRepository // 推荐、关联与发现事件的事务仓储。
	cache      HGExternalContentCache      // 可选列表缓存，不作为详情权威。
}

// NewHGExternalContentStore 构造主服务和独立crawler复用的写入适配器。
func NewHGExternalContentStore(repository HGExternalContentRepository, cache HGExternalContentCache) *HGExternalContentStore {
	return &HGExternalContentStore{repository: repository, cache: cache}
}

// UpsertRecommendations 先提交MySQL，再更新已初始化计数并失效列表分页。
func (s *HGExternalContentStore) UpsertRecommendations(ctx context.Context, items []CrawlerPlatformPackage.HGRecommendation) error {
	if len(items) == 0 {
		return nil
	}
	inserted, err := s.repository.UpsertRecommendationsWithInserted(ctx, items)
	return s.hgUpdateCacheAfterCommit(ctx, inserted, err)
}

// UpsertTaskRecommendations 原子保存内容、任务关联及可选发现事件，再维护列表缓存。
func (s *HGExternalContentStore) UpsertTaskRecommendations(ctx context.Context, taskID, runID uint64, items []CrawlerPlatformPackage.HGRecommendation) error {
	if len(items) == 0 {
		return nil
	}
	inserted, err := s.repository.UpsertTaskRecommendationsWithInserted(ctx, taskID, runID, items)
	return s.hgUpdateCacheAfterCommit(ctx, inserted, err)
}

// hgUpdateCacheAfterCommit 数据库失败不触碰缓存；Redis失败不能撤销已提交事实。
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
