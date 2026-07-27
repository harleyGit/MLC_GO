package kafka

import (
	"MLC_GO/internal/consumer"
	AuditConsumerPackage "MLC_GO/internal/consumer/audit"
	FeedConsumerPackage "MLC_GO/internal/consumer/feed"
	SearchConsumerPackage "MLC_GO/internal/consumer/search"
	StatisticConsumerPackage "MLC_GO/internal/consumer/statistic"
	HGKafkaPackage "MLC_GO/internal/pkg/kafka"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

const hgConsumerShutdownTimeout = 10 * time.Second

type hgConsumerWorker struct {
	name        string
	config      HGKafkaPackage.HGConsumerConfig
	handler     consumer.Handler
	implemented bool
	client      *kgo.Client
}

// RuntimeDependencies 是 Kafka 读模型运行期依赖。
// Redis 复用应用长生命周期连接池，避免每个 Consumer 重复创建连接池放大资源占用。
type RuntimeDependencies struct {
	Redis                      FeedConsumerPackage.RedisEvalClient
	StatisticStore             StatisticConsumerPackage.EventStore
	StatisticAggregate         StatisticConsumerPackage.AggregateReader
	StatisticRedis             StatisticConsumerPackage.RedisHashReader
	StatisticConfig            StatisticConsumerPackage.HGProjectionConfig
	StatisticReconcileConfig   StatisticConsumerPackage.HGReconcileConfig
	StatisticReconcileInterval time.Duration
}

// HGRuntime 管理各读模型独立消费组的创建、运行与关闭。
type HGRuntime struct {
	ctx               context.Context
	cancel            context.CancelFunc
	workers           []hgConsumerWorker
	wg                sync.WaitGroup
	once              sync.Once
	errMu             sync.RWMutex
	err               error
	reconciler        *StatisticConsumerPackage.HGReconciler
	reconcileInterval time.Duration
}

// NewRuntime 创建已启用的消费组；未实现消费者默认禁用，不会创建连接。
func NewRuntime(parent context.Context, cfg HGKafkaPackage.HGClusterConfig, deps RuntimeDependencies) (*HGRuntime, error) {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	runtime := &HGRuntime{ctx: ctx, cancel: cancel}
	if (cfg.Consumers.Feed.Enabled || cfg.Consumers.Statistic.Enabled) && deps.Redis == nil {
		runtime.Close()
		return nil, fmt.Errorf("redis-backed kafka consumer dependency cannot be nil")
	}
	if cfg.Consumers.Statistic.Enabled && deps.StatisticStore == nil {
		runtime.Close()
		return nil, fmt.Errorf("Statistic ClickHouse authority store dependency cannot be nil")
	}
	specs := []hgConsumerWorker{
		{
			name:        "feed",
			config:      cfg.Consumers.Feed,
			handler:     FeedConsumerPackage.NewConsumer(FeedConsumerPackage.NewRedisProjector(deps.Redis, 0, 0, "")),
			implemented: true,
		},
		{name: "search", config: cfg.Consumers.Search, handler: SearchConsumerPackage.NewConsumer()},
		{
			name:   "statistic",
			config: cfg.Consumers.Statistic,
			handler: StatisticConsumerPackage.NewConsumer(
				StatisticConsumerPackage.NewRedisCounter(deps.Redis, deps.StatisticConfig.RedisShardCount, deps.StatisticConfig.RedisGeneration),
				deps.StatisticStore,
				deps.StatisticConfig,
			),
			implemented: true,
		},
		{name: "audit", config: cfg.Consumers.Audit, handler: AuditConsumerPackage.NewConsumer()},
	}
	for _, spec := range specs {
		if !spec.config.Enabled {
			continue
		}
		if !spec.implemented {
			runtime.Close()
			return nil, fmt.Errorf("%s kafka consumer is enabled but handler is not implemented", spec.name)
		}
		opts, err := HGKafkaPackage.HGNewBusinessConsumerOpts(cfg, cfg.Topics, spec.config.GroupID, spec.config.ClientID)
		if err != nil {
			runtime.Close()
			return nil, fmt.Errorf("build %s kafka consumer: %w", spec.name, err)
		}
		spec.client, err = kgo.NewClient(opts...)
		if err != nil {
			runtime.Close()
			return nil, fmt.Errorf("new %s kafka consumer: %w", spec.name, err)
		}
		runtime.workers = append(runtime.workers, spec)
	}
	if cfg.Consumers.Statistic.Enabled && deps.StatisticAggregate != nil && deps.StatisticRedis != nil && deps.StatisticReconcileInterval > 0 {
		runtime.reconciler = StatisticConsumerPackage.NewHGReconciler(deps.StatisticAggregate, deps.StatisticRedis, deps.StatisticReconcileConfig)
		runtime.reconcileInterval = deps.StatisticReconcileInterval
	}
	return runtime, nil
}

// Start 启动所有已启用消费组。
func (r *HGRuntime) Start() {
	if r == nil {
		return
	}
	for i := range r.workers {
		worker := &r.workers[i]
		r.wg.Add(1)
		go func() {
			defer r.wg.Done()
			err := RunDomainEventConsumer(r.ctx, worker.client, "business", worker.handler)
			if err != nil && r.ctx.Err() == nil {
				r.errMu.Lock()
				if r.err == nil {
					r.err = fmt.Errorf("%s kafka consumer stopped: %w", worker.name, err)
				}
				r.errMu.Unlock()
				r.cancel()
			}
		}()
	}
	if r.reconciler != nil {
		r.wg.Add(1)
		go func() {
			defer r.wg.Done()
			ticker := time.NewTicker(r.reconcileInterval)
			defer ticker.Stop()
			for {
				select {
				case <-r.ctx.Done():
					return
				case <-ticker.C:
					_, _ = r.reconciler.Reconcile(r.ctx)
				}
			}
		}()
	}
}

// Ready 返回消费 runtime 是否仍在运行。
func (r *HGRuntime) Ready() error {
	if r == nil {
		return nil
	}
	r.errMu.RLock()
	defer r.errMu.RUnlock()
	return r.err
}

// Close 停止并等待消费者，随后关闭各自 Kafka Client。
func (r *HGRuntime) Close() {
	if r == nil {
		return
	}
	r.once.Do(func() {
		r.cancel()
		done := make(chan struct{})
		go func() {
			r.wg.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(hgConsumerShutdownTimeout):
		}
		for i := range r.workers {
			r.workers[i].client.Close()
		}
	})
}
