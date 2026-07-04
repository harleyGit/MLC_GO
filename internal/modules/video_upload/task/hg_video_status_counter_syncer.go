package VideoUploadTaskPackage

import (
	"context"
	"log"
	"time"
)

// StatusCounterRepository 提供视频状态计数的 MySQL 精确回源能力。
type StatusCounterRepository interface {
	GetVideoStatusCounts(ctx context.Context) (map[string]int64, error)
}

// StatusCounterCache 提供 Redis 视频状态计数器写入能力。
type StatusCounterCache interface {
	SetVideoStatusCounters(ctx context.Context, counters map[string]int64) error
}

// StatusCounterSyncer 定期用 MySQL 精确计数覆盖 Redis Hash，修复写侧 Redis 失败、重启或漂移导致的不一致。
type StatusCounterSyncer struct {
	repo     StatusCounterRepository
	cache    StatusCounterCache
	interval time.Duration
	done     chan struct{}
}

// NewStatusCounterSyncer 创建视频状态计数补偿器。
func NewStatusCounterSyncer(repo StatusCounterRepository, cache StatusCounterCache, interval time.Duration) *StatusCounterSyncer {
	if repo == nil || cache == nil || interval <= 0 {
		return nil
	}
	return &StatusCounterSyncer{
		repo:     repo,
		cache:    cache,
		interval: interval,
		done:     make(chan struct{}),
	}
}

// Start 启动补偿器。启动时先同步一次，避免服务刚启动 Redis 为空导致首次高并发请求集中回源 MySQL。
func (s *StatusCounterSyncer) Start(ctx context.Context) {
	if s == nil {
		return
	}
	go s.run(ctx)
}

// Stop 停止补偿器。
func (s *StatusCounterSyncer) Stop() {
	if s == nil {
		return
	}
	select {
	case <-s.done:
		return
	default:
		close(s.done)
	}
}

func (s *StatusCounterSyncer) run(ctx context.Context) {
	s.sync(ctx)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.sync(ctx)
		case <-ctx.Done():
			return
		case <-s.done:
			return
		}
	}
}

func (s *StatusCounterSyncer) sync(ctx context.Context) {
	counters, err := s.repo.GetVideoStatusCounts(ctx)
	if err != nil {
		log.Printf("video status counter sync query failed: %v", err)
		return
	}
	if err := s.cache.SetVideoStatusCounters(ctx, counters); err != nil {
		log.Printf("video status counter sync write failed: %v", err)
	}
}
