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

// HGVideoCommentMaintenanceConfig 限制单轮投影和图片清理的时间、孤儿判定年龄与批量上限。
type HGVideoCommentMaintenanceConfig struct {
	Interval, Timeout, OrphanAge time.Duration
	BatchSize                    int
}

type hgMaintenanceRepository interface {
	ProjectReactionCounts(context.Context, int) (int, error)
	ClaimImageCleanup(context.Context, time.Time, int) ([]VideoCommentRepositoryPackage.HGImageCleanupAsset, error)
	CompleteImageCleanup(context.Context, VideoCommentRepositoryPackage.HGImageCleanupAsset) error
	ReleaseImageCleanup(context.Context, string) error
}

type hgMaintenanceStorage interface {
	Delete(context.Context, string) error
}

// HGVideoCommentMaintenanceLease 为多副本部署选举单轮执行者，避免重复删除对象和放大数据库压力。
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

// RunOnce 在单个超时和全局 lease 内先投影计数，再清理一个有界图片批次。
// 图片已由 repository 从 pending/delete_pending 原子切换为 deleting，评论创建无法再绑定这些对象。
func (w *HGVideoCommentMaintenance) RunOnce(ctx context.Context) error {
	runCtx, cancel := context.WithTimeout(ctx, w.config.Timeout)
	defer cancel()
	if w.lease != nil {
		token, acquired, err := w.lease.Acquire(runCtx, hgVideoCommentMaintenanceLeaseName, w.config.Timeout+time.Second)
		if err != nil || !acquired {
			return err
		}
		defer func() { _ = w.lease.Release(context.WithoutCancel(runCtx), hgVideoCommentMaintenanceLeaseName, token) }()
	}
	if _, err := w.repository.ProjectReactionCounts(runCtx, w.config.BatchSize); err != nil {
		return err
	}
	assets, err := w.repository.ClaimImageCleanup(runCtx, w.now().UTC().Add(-w.config.OrphanAge), w.config.BatchSize)
	if err != nil {
		return err
	}
	var joined error
	for _, asset := range assets {
		if err := w.storage.Delete(runCtx, asset.StorageKey); err != nil {
			_ = w.repository.ReleaseImageCleanup(context.WithoutCancel(runCtx), asset.ImageID)
			joined = errors.Join(joined, fmt.Errorf("delete image %s: %w", asset.ImageID, err))
			continue
		}
		if err := w.repository.CompleteImageCleanup(runCtx, asset); err != nil {
			joined = errors.Join(joined, fmt.Errorf("complete image %s: %w", asset.ImageID, err))
		}
	}
	return joined
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
