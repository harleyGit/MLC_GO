package mallcny

import (
	"MLC_GO/internal/pkg/logHG"
	"context"
	"sync"
	"time"
)

// HGExpiryWorker 管理隔离沙箱订单过期、拼团失败及退款的可停止后台批次；不创建真实支付请求。
type HGExpiryWorker struct {
	cancel context.CancelFunc // 取消当前数据库操作与后续轮询。
	done   chan struct{}      // 退出通知，Close 等待完成。
	once   sync.Once          // 关闭动作幂等门闩。
}

// HGStartExpiryWorker 只在显式启用的 debug 沙箱启动；启动前必须完成授权的 000062 迁移。
func HGStartExpiryWorker(repo *HGRepository) *HGExpiryWorker {
	ctx, cancel := context.WithCancel(context.Background())
	w := &HGExpiryWorker{cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(w.done)
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				batchCtx, stop := context.WithTimeout(ctx, 4*time.Second)
				if err := repo.HGExpiryBatch(batchCtx); err != nil && ctx.Err() == nil {
					logHG.ErrFInfo("人民币沙箱商城过期与退款批次失败: %v", err)
				}
				stop()
			}
		}
	}()
	return w
}

// Close 在 MySQL 池关闭前停止 worker，不丢弃数据库已提交的补偿事实。
func (w *HGExpiryWorker) Close() {
	if w != nil {
		w.once.Do(func() { w.cancel(); <-w.done })
	}
}
