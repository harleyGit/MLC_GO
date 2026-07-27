package outbox

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const defaultMaxRetry = 10

// Producer 是 Outbox dispatcher 依赖的消息投递抽象。
// 真实环境使用 Kafka；单测或本地也可以换成内存实现。
type Producer interface {
	Send(ctx context.Context, topic string, key string, payload []byte) error
}

// Dispatcher 负责把 outbox_events 中的 pending 事件可靠投递到 Kafka。
type Dispatcher struct {
	repo     dispatcherRepository
	producer Producer
	maxRetry int
	lease    time.Duration
	workers  int
}

type dispatcherRepository interface {
	Claim(ctx context.Context, limit int, leaseDuration time.Duration) ([]Event, error)
	MarkPublished(ctx context.Context, id int64, leaseToken string) (bool, error)
	MarkRetry(ctx context.Context, id int64, leaseToken string, reason string, delay time.Duration) (bool, error)
	MarkDead(ctx context.Context, id int64, leaseToken string, reason string) (bool, error)
}

// NewDispatcher 创建 Outbox 投递器。
func NewDispatcher(repo *Repository, producer Producer) *Dispatcher {
	return NewDispatcherWithRepository(repo, producer)
}

// NewDispatcherWithRepository 创建支持测试替身的短事务 Outbox dispatcher。
func NewDispatcherWithRepository(repo dispatcherRepository, producer Producer) *Dispatcher {
	return &Dispatcher{repo: repo, producer: producer, maxRetry: defaultMaxRetry, lease: 30 * time.Second, workers: 8}
}

// DispatchOnce 拉取并处理一批 Outbox 事件。
// 成功：标记 published；失败：标记 retry；超过最大次数：标记 dead，等待人工排查或补偿。
func (d *Dispatcher) DispatchOnce(ctx context.Context, batchSize int) error {
	if d == nil || d.repo == nil {
		return fmt.Errorf("outbox dispatcher repository cannot be nil")
	}
	if d.producer == nil {
		return fmt.Errorf("outbox dispatcher producer cannot be nil")
	}

	if batchSize <= 0 || batchSize > d.workers {
		batchSize = d.workers
	}
	events, err := d.repo.Claim(ctx, batchSize, d.lease)
	if err != nil {
		return err
	}
	var wg sync.WaitGroup
	errCh := make(chan error, len(events))
	for _, event := range events {
		event := event
		wg.Add(1)
		go func() {
			defer wg.Done()
			var markErr error
			if sendErr := d.producer.Send(ctx, event.Topic, event.EventKey, event.Payload); sendErr != nil {
				if event.RetryCount+1 >= d.maxRetry {
					_, markErr = d.repo.MarkDead(ctx, event.ID, event.LeaseToken, sendErr.Error())
				} else {
					_, markErr = d.repo.MarkRetry(ctx, event.ID, event.LeaseToken, sendErr.Error(), hgOutboxRetryDelay(event.RetryCount))
				}
			} else {
				_, markErr = d.repo.MarkPublished(ctx, event.ID, event.LeaseToken)
			}
			if markErr != nil {
				errCh <- markErr
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for dispatchErr := range errCh {
		return dispatchErr
	}
	return nil
}

func hgOutboxRetryDelay(retryCount int) time.Duration {
	if retryCount < 0 {
		retryCount = 0
	}
	if retryCount > 8 {
		retryCount = 8
	}
	return time.Second * time.Duration(1<<retryCount)
}

// Run 按固定间隔持续投递 Outbox，通常由 scheduler 或应用启动钩子拉起。
func (d *Dispatcher) Run(ctx context.Context, interval time.Duration, batchSize int) error {
	if interval <= 0 {
		// 默认 1 秒轮询，兼顾本地开发实时性和数据库压力。
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		// 每轮独立开启事务；失败直接返回，交给上层 supervisor 决定重启或告警。
		if err := d.DispatchOnce(ctx, batchSize); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
