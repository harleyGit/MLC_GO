package media

import (
	"MLC_GO/internal/pkg/logHG"
	"context"
	"errors"
	"sync"
	"time"
)

// HGRuntime 管理固定数量后台 worker；多 Pod 通过 MySQL SKIP LOCKED + 租约竞争，不使用内存任务队列。
type HGRuntime struct {
	service       *HGService         // 审核、过期与删除共享权威 MySQL 和同一桶存储客户端。
	closeReviewer func()             // 停止后关闭审核 HTTP 空闲连接；不在运行时热切换。
	stop          context.CancelFunc // 启动后停止 worker，nil 表示尚未启动。
	mu            sync.Mutex         // 保护 stop、closed 和最后一次后台错误。
	wg            sync.WaitGroup     // 等待固定 worker 完全退出后才关闭 MySQL。
	closed        bool               // Close 后禁止再次启动。
	errors        map[string]bool    // 每类 worker 的最近故障状态，不含敏感参数。
}

// HGStart 在应用依赖全部就绪后启动固定数量 worker，后台上下文在 Close 时取消。
func (r *HGRuntime) HGStart() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || r.stop != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.stop = cancel
	r.errors = make(map[string]bool, 4)
	r.hgLoop(ctx, "review", time.Second, 280*time.Second, r.service.HGReviewOnce)
	r.hgLoop(ctx, "expire", time.Second, 10*time.Second, r.service.HGExpireOnce)
	r.hgLoop(ctx, "delete", time.Second, 15*time.Second, r.service.HGDeleteOnce)
	r.hgLoop(ctx, "repair", time.Minute, 10*time.Second, r.service.hgRecoverFinalized)
}

func (r *HGRuntime) hgLoop(ctx context.Context, name string, interval, budget time.Duration, run func(context.Context) error) {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		lastLog := time.Time{}
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				work, cancel := context.WithTimeout(ctx, budget)
				err := run(work)
				cancel()
				r.mu.Lock()
				r.errors[name] = err != nil
				r.mu.Unlock()
				if err != nil && time.Since(lastLog) >= time.Minute {
					// S3 SDK 错误可能含带签名 URL；这里只记录固定任务类型，不输出原始错误。
					logHG.ErrFInfo("媒体后台任务失败，任务类型=%s", name)
					lastLog = time.Now()
				}
			}
		}
	}()
}

// Ready 提供后台故障可见性；最近一次成功只说明 worker 正常，不代表审核/SeaweedFS 容量验收。
func (r *HGRuntime) Ready() error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || r.stop == nil {
		return errors.New("媒体后台尚未启动或已关闭")
	}
	for _, failed := range r.errors {
		if failed {
			return errors.New("媒体后台存在未恢复错误")
		}
	}
	return nil
}

// Close 取消审核、清理和恢复任务，并等待下游调用退出；必须早于 MySQL 池关闭。
func (r *HGRuntime) Close() {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.closed = true
	stop := r.stop
	r.mu.Unlock()
	if stop != nil {
		stop()
	}
	r.wg.Wait()
	if r.closeReviewer != nil {
		r.closeReviewer()
	}
}
