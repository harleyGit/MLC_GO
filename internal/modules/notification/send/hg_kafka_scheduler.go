package send

import (
	"MLC_GO/internal/pkg/logHG"
	"context"
	"errors"
	"sync"
	"time"
)

// HGKafkaScheduler 是每 Pod 一个有界到期扫描器；只做 MySQL+Outbox 短事务，不直接发送通知。
type HGKafkaScheduler struct {
	repo   *HGRepository      // 共享连接池；不负责关闭。
	cancel context.CancelFunc // 停止轮询；尚未启动时为空。
	done   chan struct{}      // 等待最后一个短事务完成；尚未启动时为空。
	once   sync.Once          // 重复关闭不会阻塞。
}

// HGNewKafkaScheduler 构建在 Kafka 消费组已验证配置后才允许启动的调度器。
func HGNewKafkaScheduler(repo *HGRepository) *HGKafkaScheduler { return &HGKafkaScheduler{repo: repo} }

// HGStart 每秒至多为一个当前游标生成出站事件；背压由 MySQL 有界扫描与 Outbox 承担。
func (scheduler *HGKafkaScheduler) HGStart() {
	ctx, cancel := context.WithCancel(context.Background())
	scheduler.cancel, scheduler.done = cancel, make(chan struct{})
	go func() {
		defer close(scheduler.done)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		lastLog := time.Time{}
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				callCtx, stop := context.WithTimeout(ctx, 5*time.Second)
				_, err := scheduler.repo.HGQueueKafkaOnce(callCtx)
				stop()
				if err != nil && time.Since(lastLog) >= time.Minute {
					// 不记录异常正文、出站 payload、用户 ID 或任务变量。
					logHG.ErrFInfo("通知 Kafka 到期入队失败（限速日志）：%T", err)
					lastLog = time.Now()
				}
			}
		}
	}()
}

// Close 等待当前到期事务退出，必须先于共享 MySQL 和 Outbox dispatcher 关闭。
func (scheduler *HGKafkaScheduler) Close() {
	if scheduler == nil {
		return
	}
	scheduler.once.Do(func() {
		if scheduler.cancel != nil {
			scheduler.cancel()
			<-scheduler.done
		}
	})
}

// HGReady 为上层显式检查调度依赖；实际 Kafka 消费健康由运行时独立负责。
func (scheduler *HGKafkaScheduler) HGReady() error {
	if scheduler == nil || scheduler.repo == nil || scheduler.repo.db == nil || scheduler.cancel == nil {
		return errors.New("通知 Kafka 到期调度器未启动")
	}
	return nil
}
