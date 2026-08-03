package VideoCommentTaskPackage

import (
	VideoCommentRepositoryPackage "MLC_GO/internal/modules/video_comment/repository"
	"MLC_GO/internal/pkg/logHG"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

const hgVideoCommentMaintenanceLeaseName = "video_comment_maintenance"

// 指标刷新使用独立短超时，数据库观测异常不能耗尽投影和对象清理的主执行窗口。
const hgVideoCommentMaintenanceMetricTimeout = 2 * time.Second

// HGVideoCommentMaintenanceConfig 限制单轮投影和图片清理的时间、孤儿判定年龄与批量上限。
type HGVideoCommentMaintenanceConfig struct {
	Interval, Timeout, OrphanAge time.Duration
	BatchSize                    int
}

type hgMaintenanceRepository interface {
	ProjectReactionCounts(context.Context, int) (VideoCommentRepositoryPackage.HGReactionProjectionResult, error)
	ClaimImageCleanup(context.Context, time.Time, int, time.Duration) (VideoCommentRepositoryPackage.HGImageCleanupClaim, error)
	CompleteImageCleanup(context.Context, VideoCommentRepositoryPackage.HGImageCleanupAsset) error
	ReleaseImageCleanup(context.Context, VideoCommentRepositoryPackage.HGImageCleanupAsset) error
	MaintenanceOldestTimes(context.Context, time.Time, time.Duration) (time.Time, time.Time, error)
}

type hgMaintenanceStorage interface {
	Delete(context.Context, string) error
}

// HGVideoCommentMaintenanceLease 为多副本部署选举单轮执行者，避免正常情况下重复扫描和放大数据库压力。
// Redis lease 只负责削峰，不承担图片清理正确性；真正的崩溃恢复和陈旧执行者隔离由 MySQL 行租约与 fencing token 保证。
type HGVideoCommentMaintenanceLease interface {
	Acquire(context.Context, string, time.Duration) (string, bool, error)
	Release(context.Context, string, string) error
}

// HGVideoCommentMaintenance 串行执行赞踩投影和已原子 claim 图片的对象清理。
type HGVideoCommentMaintenance struct {
	repository hgMaintenanceRepository
	storage    hgMaintenanceStorage
	lease      HGVideoCommentMaintenanceLease
	config     HGVideoCommentMaintenanceConfig
	now        func() time.Time
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	mu         sync.Mutex
}

// NewHGVideoCommentMaintenance 校验严格容量边界；lease 可选，生产多副本应传入共享 Redis lease。
func NewHGVideoCommentMaintenance(repository hgMaintenanceRepository, storage hgMaintenanceStorage, config HGVideoCommentMaintenanceConfig, leases ...HGVideoCommentMaintenanceLease) (*HGVideoCommentMaintenance, error) {
	if repository == nil || storage == nil || config.Interval <= 0 || config.Timeout <= 0 || config.Timeout >= config.Interval || config.OrphanAge <= config.Timeout || config.BatchSize < 1 || config.BatchSize > 1000 {
		return nil, errors.New("video comment maintenance configuration is invalid")
	}
	var lease HGVideoCommentMaintenanceLease
	if len(leases) > 0 {
		lease = leases[0]
	}
	return &HGVideoCommentMaintenance{repository: repository, storage: storage, lease: lease, config: config, now: time.Now}, nil
}

// Start 幂等启动一次立即执行和后续周期执行的可取消 goroutine。
func (w *HGVideoCommentMaintenance) Start(parent context.Context) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(parent)
	w.cancel = cancel
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		defer func() {
			if recovered := recover(); recovered != nil {
				logHG.ErrFInfo("Video comment maintenance panic: %v", recovered)
			}
		}()
		w.hgRunAndLog(ctx)
		ticker := time.NewTicker(w.config.Interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.hgRunAndLog(ctx)
			}
		}
	}()
}

func (w *HGVideoCommentMaintenance) hgRunAndLog(ctx context.Context) {
	if err := w.RunOnce(ctx); err != nil && ctx.Err() == nil {
		logHG.ErrFInfo("Video comment maintenance failed: %v", err)
	}
}

// RunOnce 在单个超时和全局 lease 内有界排空计数投影，并独立清理一个图片批次。
// 投影错误会被收集但不会阻断图片清理，避免无关计数故障导致对象和用户配额持续积压。
// 图片已由 repository 原子切换为 deleting；对象存储 I/O 在数据库事务外执行，避免持锁等待网络。
func (w *HGVideoCommentMaintenance) RunOnce(ctx context.Context) error {
	now := w.now().UTC()
	orphanBefore := now.Add(-w.config.OrphanAge)
	// 每个副本都刷新进程内 age gauge，避免 Redis lease owner 切换后旧副本长期暴露陈旧值；查询均为索引 LIMIT 1。
	metricTimeout := min(w.config.Timeout/4, hgVideoCommentMaintenanceMetricTimeout)
	metricCtx, metricCancel := context.WithTimeout(ctx, metricTimeout)
	dirtyOldest, cleanupOldest, metricErr := w.repository.MaintenanceOldestTimes(metricCtx, orphanBefore, w.config.OrphanAge)
	metricCancel()
	if metricErr == nil {
		hgReactionDirtyOldestAgeSeconds.Store(hgAgeSeconds(now, dirtyOldest))
		hgImageCleanupOldestAgeSeconds.Store(hgAgeSeconds(now, cleanupOldest))
	}
	// 维护主 timeout 从指标刷新后独立开始，观测失败只作为 joined error 返回，不阻断后续有界工作。
	runCtx, cancel := context.WithTimeout(ctx, w.config.Timeout)
	defer cancel()
	if w.lease != nil {
		token, acquired, err := w.lease.Acquire(runCtx, hgVideoCommentMaintenanceLeaseName, w.config.Timeout+time.Second)
		if err != nil || !acquired {
			return err
		}
		defer func() { _ = w.lease.Release(context.WithoutCancel(runCtx), hgVideoCommentMaintenanceLeaseName, token) }()
	}
	var joined error
	if metricErr != nil {
		joined = errors.Join(joined, metricErr)
	}
	// 单轮最多处理 10 个批次，在追赶 dirty backlog 的同时限制数据库占用；runCtx 提供第二层总时限。
	for batch := 0; batch < 10; batch++ {
		projection, err := w.repository.ProjectReactionCounts(runCtx, w.config.BatchSize)
		hgReactionProjectionCASMisses.Add(uint64(projection.CASMisses))
		if err != nil {
			joined = errors.Join(joined, err)
			break
		}
		if projection.Projected < w.config.BatchSize {
			break
		}
	}
	claim, err := w.repository.ClaimImageCleanup(runCtx, orphanBefore, w.config.BatchSize, w.config.Timeout+time.Second)
	if err != nil {
		return errors.Join(joined, err)
	}
	hgExpiredLeaseReclaims.Add(uint64(claim.ExpiredLeaseReclaims))
	for _, asset := range claim.Assets {
		if err := w.storage.Delete(runCtx, asset.StorageKey); err != nil {
			// DELETE 超时属于结果不确定，恢复为不可绑定的 delete_pending；后续轮次使用新 token 幂等重试。
			hgImageCleanupFailures.Add(1)
			joined = errors.Join(joined, fmt.Errorf("delete image %s: %w", asset.ImageID, err))
			if releaseErr := w.repository.ReleaseImageCleanup(context.WithoutCancel(runCtx), asset); releaseErr != nil {
				// 存储失败与状态恢复失败是两个独立故障，分别计数并返回，便于区分对象存储和 MySQL 恢复链路异常。
				hgImageCleanupFailures.Add(1)
				joined = errors.Join(joined, fmt.Errorf("release image %s: %w", asset.ImageID, releaseErr))
			}
			continue
		}
		if err := w.repository.CompleteImageCleanup(runCtx, asset); err != nil {
			hgImageCleanupFailures.Add(1)
			joined = errors.Join(joined, fmt.Errorf("complete image %s: %w", asset.ImageID, err))
		}
	}
	return joined
}

// hgAgeSeconds 返回从条目达到可处理条件开始的整秒积压年龄；空队列或未来时间统一为 0。
func hgAgeSeconds(now, oldest time.Time) uint64 {
	if oldest.IsZero() || !oldest.Before(now) {
		return 0
	}
	return uint64(now.Sub(oldest) / time.Second)
}

// Close 取消 worker 并等待当前有界任务退出，调用方应在关闭 Redis、MySQL 前执行。
func (w *HGVideoCommentMaintenance) Close() {
	w.mu.Lock()
	cancel := w.cancel
	w.cancel = nil
	w.mu.Unlock()
	if cancel != nil {
		cancel()
		w.wg.Wait()
	}
}
