package product

import (
	"MLC_GO/internal/pkg/logHG"
	"context"
	"sync"
	"time"
)

// HGExpiryWorker 每5秒分批释放过期实物库存；多 Pod 通过订单行锁安全重试。
type HGExpiryWorker struct {
	cancel context.CancelFunc // 关闭信号；停止新批次并取消当前数据库调用。
	done   chan struct{}      // worker 退出通知，关闭前等待数据库调用结束。
	once   sync.Once          // Close 幂等保护，避免重复关闭资源。
}

// HGStartExpiryWorker 在服务装配完成后启动有界过期任务；数据库错误会记录并在下轮重试。
func HGStartExpiryWorker(repo *HGOrderRepository) *HGExpiryWorker {
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
				if err := repo.HGExpireBatch(batchCtx); err != nil && ctx.Err() == nil {
					logHG.ErrFInfo("商城过期待支付库存释放失败: %v", err)
				}
				stop()
			}
		}
	}()
	return w
}

// Close 停止过期处理并等待 worker 退出，须先于 MySQL 连接池关闭。
func (w *HGExpiryWorker) Close() {
	if w != nil {
		w.once.Do(func() { w.cancel(); <-w.done })
	}
}
