package VideoInteractionTaskPackage

import (
	VideoInteractionRepositoryPackage "MLC_GO/internal/modules/video_interaction/repository"
	"MLC_GO/internal/pkg/logHG"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// HGMaxProjectionPageSize 是每条流的硬上限，配置值不能扩大单轮 MySQL/Redis 压力。
const HGMaxProjectionPageSize = 1000

// HGProjectionStream 标识四条固定、低基数修复流；不得动态使用 user_id 或 submission_id 作为 stream。
type HGProjectionStream string

const (
	HGProjectionStreamVideoState   HGProjectionStream = "video_state"
	HGProjectionStreamFollowState  HGProjectionStream = "follow_state"
	HGProjectionStreamVideoCounts  HGProjectionStream = "video_counts"
	HGProjectionStreamFollowCounts HGProjectionStream = "follow_counts"
)

var hgProjectionStreams = [...]HGProjectionStream{
	HGProjectionStreamVideoState, HGProjectionStreamFollowState, HGProjectionStreamVideoCounts, HGProjectionStreamFollowCounts,
}

// HGReprojectConfig 定义调度、单轮超时、写入安全延迟、lease TTL 和分页上限。
// Timeout 必须小于 LeaseTTL 和 Interval，确保 owner 正常情况下不会在租约过期后继续提交 checkpoint。
type HGReprojectConfig struct {
	Interval  time.Duration
	Timeout   time.Duration
	SafetyLag time.Duration
	LeaseTTL  time.Duration
	PageSize  int
}

// HGProjectionRepository 是 worker 所需的最小 MySQL keyset 读取接口；实现禁止 OFFSET 和无界结果集。
type HGProjectionRepository interface {
	ListVideoStates(context.Context, VideoInteractionRepositoryPackage.HGProjectionCursor, time.Time, int) ([]VideoInteractionRepositoryPackage.HGVideoStateProjection, error)
	ListFollowStates(context.Context, VideoInteractionRepositoryPackage.HGProjectionCursor, time.Time, int) ([]VideoInteractionRepositoryPackage.HGFollowStateProjection, error)
	ListVideoCounts(context.Context, VideoInteractionRepositoryPackage.HGProjectionCursor, time.Time, int) ([]VideoInteractionRepositoryPackage.HGVideoCountProjection, error)
	ListFollowCounts(context.Context, VideoInteractionRepositoryPackage.HGProjectionCursor, time.Time, int) ([]VideoInteractionRepositoryPackage.HGFollowCountProjection, error)
}

// HGProjectionCache 是带 owner fencing 的 Redis 投影接口。
// 绝对 HSET 成功后才能提交 checkpoint；失败页不推进游标，重放仍保持幂等。
type HGProjectionCache interface {
	AcquireLease(context.Context, string, time.Duration) (string, bool, error)
	LoadCheckpoint(context.Context, string) (string, error)
	ApplyVideoStates(context.Context, []VideoInteractionRepositoryPackage.HGVideoStateProjection) error
	ApplyFollowStates(context.Context, []VideoInteractionRepositoryPackage.HGFollowStateProjection) error
	ApplyVideoCounts(context.Context, []VideoInteractionRepositoryPackage.HGVideoCountProjection) error
	ApplyFollowCounts(context.Context, []VideoInteractionRepositoryPackage.HGFollowCountProjection) error
	CommitCheckpoint(context.Context, string, string, string) error
	ReleaseLease(context.Context, string, string) error
}

// HGReprojector 定期把 MySQL 权威互动状态修复到 Redis，并只持有一个可取消 goroutine。
// 每轮四条流各处理一页，避免补偿任务在数据积压时抢占在线请求的数据库和 Redis 容量。
type HGReprojector struct {
	repository HGProjectionRepository
	cache      HGProjectionCache
	config     HGReprojectConfig
	hgNow      func() time.Time
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	mu         sync.Mutex
}

// NewHGReprojector 校验生产边界并创建停止状态的受管 worker。
func NewHGReprojector(repository HGProjectionRepository, cache HGProjectionCache, config HGReprojectConfig) (*HGReprojector, error) {
	if repository == nil || cache == nil || config.Interval <= 0 || config.Timeout <= 0 || config.Timeout >= config.Interval || config.SafetyLag <= 0 || config.LeaseTTL <= config.Timeout || config.PageSize <= 0 {
		return nil, errors.New("interaction reprojector configuration is invalid")
	}
	if config.PageSize > HGMaxProjectionPageSize {
		config.PageSize = HGMaxProjectionPageSize
	}
	return &HGReprojector{repository: repository, cache: cache, config: config, hgNow: time.Now}, nil
}

// Start 启动受管周期 worker；重复调用不会创建重叠 goroutine。
func (r *HGReprojector) Start(parent context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(parent)
	r.cancel = cancel
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		defer func() {
			if recovered := recover(); recovered != nil {
				logHG.ErrFInfo("Interaction reprojector panic: %v", recovered)
			}
		}()
		ticker := time.NewTicker(r.config.Interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := r.hgRunSafely(ctx); err != nil && ctx.Err() == nil {
					logHG.ErrFInfo("Interaction reprojector run failed: %v", err)
				}
			}
		}
	}()
}

func (r *HGReprojector) hgRunSafely(ctx context.Context) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("interaction reprojector panic: %v", recovered)
		}
	}()
	return r.RunOnce(ctx)
}

// Close 取消 worker 并等待当前有界轮次退出，必须在关闭 Redis/MySQL 连接池之前调用。
func (r *HGReprojector) Close() {
	r.mu.Lock()
	cancel := r.cancel
	r.cancel = nil
	r.mu.Unlock()
	if cancel != nil {
		cancel()
		r.wg.Wait()
	}
}

// RunOnce 为每条固定流最多处理一个有界页面；SafetyLag 避免覆盖仍在 Kafka/Outbox 一致性窗口中的乐观值。
func (r *HGReprojector) RunOnce(ctx context.Context) error {
	runCtx, cancel := context.WithTimeout(ctx, r.config.Timeout)
	defer cancel()
	cutoff := r.hgNow().UTC().Add(-r.config.SafetyLag)
	var joined error
	for _, stream := range hgProjectionStreams {
		if err := r.hgRunStream(runCtx, stream, cutoff); err != nil {
			joined = errors.Join(joined, fmt.Errorf("reproject %s: %w", stream, err))
		}
	}
	return joined
}

func (r *HGReprojector) hgRunStream(ctx context.Context, stream HGProjectionStream, cutoff time.Time) error {
	started := time.Now()
	token, acquired, err := r.cache.AcquireLease(ctx, string(stream), r.config.LeaseTTL)
	if err != nil || !acquired {
		hgObserveProjection(stream, 0, time.Since(started), err, acquired)
		return err
	}
	released := false
	defer func() {
		if !released {
			_ = r.cache.ReleaseLease(context.WithoutCancel(ctx), string(stream), token)
		}
	}()
	rawCursor, err := r.cache.LoadCheckpoint(ctx, string(stream))
	if err != nil {
		return err
	}
	var cursor VideoInteractionRepositoryPackage.HGProjectionCursor
	if rawCursor != "" {
		if err := json.Unmarshal([]byte(rawCursor), &cursor); err != nil {
			return fmt.Errorf("decode checkpoint: %w", err)
		}
	} else {
		cursor.UpdatedAt = time.Unix(0, 0).UTC()
	}
	processed, next, err := r.hgProjectPage(ctx, stream, cursor, cutoff)
	if err != nil {
		hgObserveProjection(stream, processed, time.Since(started), err, true)
		return err
	}
	// 无论页面是否填满，只要成功写入记录就保存最后复合游标；尾页清空游标会导致亿级表每轮从头扫描。
	nextCheckpoint := rawCursor
	if processed > 0 {
		encoded, encodeErr := json.Marshal(next)
		if encodeErr != nil {
			return fmt.Errorf("encode checkpoint: %w", encodeErr)
		}
		nextCheckpoint = string(encoded)
	}
	if err := r.cache.CommitCheckpoint(ctx, string(stream), token, nextCheckpoint); err != nil {
		hgObserveProjection(stream, processed, time.Since(started), err, true)
		return err
	}
	released = true
	hgObserveProjection(stream, processed, time.Since(started), nil, true)
	return nil
}

func (r *HGReprojector) hgProjectPage(ctx context.Context, stream HGProjectionStream, cursor VideoInteractionRepositoryPackage.HGProjectionCursor, cutoff time.Time) (int, VideoInteractionRepositoryPackage.HGProjectionCursor, error) {
	switch stream {
	case HGProjectionStreamVideoState:
		rows, err := r.repository.ListVideoStates(ctx, cursor, cutoff, r.config.PageSize)
		if err != nil || len(rows) == 0 {
			return len(rows), cursor, err
		}
		if err := r.cache.ApplyVideoStates(ctx, rows); err != nil {
			return len(rows), cursor, err
		}
		return len(rows), rows[len(rows)-1].Cursor, nil
	case HGProjectionStreamFollowState:
		rows, err := r.repository.ListFollowStates(ctx, cursor, cutoff, r.config.PageSize)
		if err != nil || len(rows) == 0 {
			return len(rows), cursor, err
		}
		if err := r.cache.ApplyFollowStates(ctx, rows); err != nil {
			return len(rows), cursor, err
		}
		return len(rows), rows[len(rows)-1].Cursor, nil
	case HGProjectionStreamVideoCounts:
		rows, err := r.repository.ListVideoCounts(ctx, cursor, cutoff, r.config.PageSize)
		if err != nil || len(rows) == 0 {
			return len(rows), cursor, err
		}
		if err := r.cache.ApplyVideoCounts(ctx, rows); err != nil {
			return len(rows), cursor, err
		}
		return len(rows), rows[len(rows)-1].Cursor, nil
	case HGProjectionStreamFollowCounts:
		rows, err := r.repository.ListFollowCounts(ctx, cursor, cutoff, r.config.PageSize)
		if err != nil || len(rows) == 0 {
			return len(rows), cursor, err
		}
		if err := r.cache.ApplyFollowCounts(ctx, rows); err != nil {
			return len(rows), cursor, err
		}
		return len(rows), rows[len(rows)-1].Cursor, nil
	default:
		return 0, cursor, fmt.Errorf("unknown projection stream %q", stream)
	}
}
