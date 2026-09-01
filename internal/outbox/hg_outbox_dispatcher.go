package outbox

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const defaultMaxRetry = 10
const defaultDispatcherWorkers = 64

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

	// Claim 查询 outbox 表，捞取满足条件的待发送记录； 更新记录：写入`LeaseToken`、设置租约过期时间；
	// 在同一个 SQL 内完成查询 + 更新，数据库行锁保证，别的实例 claim 不到这条记录
	// 
	//	@param ctx 
	//	@param limit 
	//	@param leaseDuration 租约有效期。如果 worker 崩溃，租约到期后，其他 dispatcher 可以重新 claim 这条消息，避免消息永久卡住。
	//	@return []Event 
	//	@return error 
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
	return &Dispatcher{repo: repo, producer: producer, maxRetry: defaultMaxRetry, lease: 30 * time.Second, workers: defaultDispatcherWorkers}
}

// DispatchOnce 拉取并处理一批 Outbox 事件。
// 成功：标记 published；失败：标记 retry；超过最大次数：标记 dead，等待人工排查或补偿。
func (d *Dispatcher) DispatchOnce(ctx context.Context, batchSize int) error {
	_, err := d.hgDispatchAvailable(ctx, batchSize)
	return err
}

// hgDispatchAvailable Outbox（发件箱模式）的调度发送核心函数
// 整体逻辑：抢占一批待发送 outbox 记录 → 多 goroutine 并发发送 kafka → 根据发送结果标记数据库记录（已发布 / 重试 / 死信）。
//
//	@param ctx
//	@param batchSize
//	@return int
//	@return error
func (d *Dispatcher) hgDispatchAvailable(ctx context.Context, batchSize int) (int, error) {

	// repo：outbox 数据库存储层（操作 outbox 表）
	if d == nil || d.repo == nil {
		return 0, fmt.Errorf("outbox dispatcher repository cannot be nil")
	}

	// producer：kafka 生产者，把 outbox 里的 Envelope 消息发送到 kafka
	if d.producer == nil {
		return 0, fmt.Errorf("outbox dispatcher producer cannot be nil")
	}

	// 批量大小保护：不能大于worker并发数； 一次捞取的 outbox 条数不能超过工作协程数量，防止启动大量 goroutine。
	if batchSize <= 0 || batchSize > d.workers {
		batchSize = d.workers
	}
	// 1. 抢占(claim)一批待发送outbox记录，带上租约lease
	// outbox 多实例部署，多个 worker 进程不能重复处理同一条 outbox 记录，用**数据库租约 (lease)**实现分布式锁。
	events, err := d.repo.Claim(ctx, batchSize, d.lease)
	if err != nil {
		return 0, err
	}
	var wg sync.WaitGroup
	errCh := make(chan error, len(events))
	// 2. 每条outbox记录启动goroutine并发发送kafka
	for _, event := range events {
		event := event
		wg.Add(1)
		go func() {
			defer wg.Done()
			var markErr error
			// 调用producer发送kafka，payload就是完整EventEnvelope json
			if sendErr := d.producer.Send(ctx, event.Topic, event.EventKey, event.Payload); sendErr != nil {
				// kafka发送失败
				if event.RetryCount+1 >= d.maxRetry {
					// 超过最大重试次数 → 标记死信 MarkDead，不再重试
					_, markErr = d.repo.MarkDead(ctx, event.ID, event.LeaseToken, sendErr.Error())
				} else {
					// 还没到最大重试次数 → MarkRetry，增加重试计数，设置延迟下次再捞
					_, markErr = d.repo.MarkRetry(ctx, event.ID, event.LeaseToken, sendErr.Error(), hgOutboxRetryDelay(event.RetryCount))
				}
			} else {
				// ✅发送kafka成功，标记这条outbox记录已发布
				_, markErr = d.repo.MarkPublished(ctx, event.ID, event.LeaseToken)
			}
			// 如果标记DB操作出错，把错误丢进errCh
			if markErr != nil {
				errCh <- markErr
			}
		}()
	}
	// 等待所有并发发送goroutine全部完成
	wg.Wait()
	close(errCh)
	// 只要有任意一条记录的标记DB出错，直接返回该错误
	for dispatchErr := range errCh {
		return len(events), dispatchErr
	}
	// 成功处理这批，返回本次取出多少条outbox事件
	return len(events), nil
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
		// 有积压时持续以固定并发排空，只有当前批次为空才等待轮询间隔。
		processed, err := d.hgDispatchAvailable(ctx, batchSize)
		if err != nil {
			return err
		}
		if processed > 0 {
			continue
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
