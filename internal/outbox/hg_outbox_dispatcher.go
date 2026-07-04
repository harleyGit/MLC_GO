package outbox

import (
	"context"
	"database/sql"
	"fmt"
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
	repo     *Repository
	producer Producer
	maxRetry int
}

// NewDispatcher 创建 Outbox 投递器。
func NewDispatcher(repo *Repository, producer Producer) *Dispatcher {
	return &Dispatcher{repo: repo, producer: producer, maxRetry: defaultMaxRetry}
}

// DispatchOnce 拉取并处理一批 Outbox 事件。
// 成功：标记 published；失败：标记 retry；超过最大次数：标记 dead，等待人工排查或补偿。
func (d *Dispatcher) DispatchOnce(ctx context.Context, batchSize int) error {
	if d == nil || d.repo == nil || d.repo.dbConn() == nil {
		return fmt.Errorf("outbox dispatcher repository cannot be nil")
	}
	if d.producer == nil {
		return fmt.Errorf("outbox dispatcher producer cannot be nil")
	}

	// 每批事件放在同一个事务内完成锁定和状态更新，避免多个 dispatcher 重复处理同一批行。
	tx, err := d.repo.dbConn().BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	// Commit 成功后 Rollback 会返回 sql.ErrTxDone；这里忽略即可，保证异常路径释放事务。
	defer tx.Rollback()

	events, err := d.repo.FetchPendingTx(ctx, tx, batchSize)
	if err != nil {
		return err
	}
	for _, event := range events {
		// 先投递 Kafka，再更新本地状态；若状态更新失败，下一轮可能重复投递，所以消费侧必须按 EventID/EventKey 做幂等。
		if err := d.producer.Send(ctx, event.Topic, event.EventKey, event.Payload); err != nil {
			if event.RetryCount+1 >= d.maxRetry {
				// 达到上限后转 dead，避免坏消息一直占用 dispatcher 处理能力。
				if markErr := d.repo.MarkDeadTx(ctx, tx, event.ID, err.Error()); markErr != nil {
					return markErr
				}
				continue
			}
			if markErr := d.repo.MarkRetryTx(ctx, tx, event.ID, err.Error()); markErr != nil {
				return markErr
			}
			continue
		}
		// Kafka 已确认写入后再标记 published。
		if err := d.repo.MarkPublishedTx(ctx, tx, event.ID); err != nil {
			return err
		}
	}

	return tx.Commit()
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
