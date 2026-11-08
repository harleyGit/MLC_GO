package commerce

import (
	"MLC_GO/internal/pkg/logHG"
	"context"
	"sync"
	"time"
)

// HGExpiryWorker 只管理隔离 debug 已开启的待支付订单过期补偿；停止时等待当前批次结束。
type HGExpiryWorker struct {
	cancel context.CancelFunc // 取消后续轮询与正在运行的有界 DB 查询。
	done   chan struct{}      // 工作协程退出信号。
	once   sync.Once          // 确保并发停止幂等。
}

// HGStartExpiryWorker 只由完成 debug 配置及迁移验证的装配层调用；批次20单，单次超时4秒。
func HGStartExpiryWorker(store *HGOrderStore) *HGExpiryWorker {
	return hgStartExpiryWorker(store, 5*time.Second)
}

// hgStartExpiryWorker 仅供隔离测试缩短轮询间隔；单批超时和数量不随间隔变化。
func hgStartExpiryWorker(store *HGOrderStore, interval time.Duration) *HGExpiryWorker {
	ctx, cancel := context.WithCancel(context.Background())
	w := &HGExpiryWorker{cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(w.done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				batch, stop := context.WithTimeout(ctx, 4*time.Second)
				if err := store.HGExpiryBatch(batch); err != nil && ctx.Err() == nil {
					logHG.ErrFInfo("生产人民币商品 debug 订单过期补偿失败: %v", err)
				}
				stop()
			}
		}
	}()
	return w
}

// Close 停止并等待当前轮询；必须在共享 MySQL 池关闭前调用。
func (w *HGExpiryWorker) Close() {
	if w != nil {
		w.once.Do(func() { w.cancel(); <-w.done })
	}
}
