package kafka

import (
	"MLC_GO/internal/consumer"
	AuditConsumerPackage "MLC_GO/internal/consumer/audit"
	DanmakuConsumerPackage "MLC_GO/internal/consumer/danmaku"
	FeedConsumerPackage "MLC_GO/internal/consumer/feed"
	InteractionConsumerPackage "MLC_GO/internal/consumer/interaction"
	SearchConsumerPackage "MLC_GO/internal/consumer/search"
	StatisticConsumerPackage "MLC_GO/internal/consumer/statistic"
	HGKafkaPackage "MLC_GO/internal/pkg/kafka"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

/* Kafka 读模型运行时。作用：
这是一个 Kafka 多消费组运行时管理器（HGRuntime），统一管理多个业务读模型（Feed / Search / Statistic / Audit / Interaction / Danmaku）的 Kafka 消费者生命周期：创建 → 启动 → 健康检查 → 优雅关闭，并附带 Lag 监控和周期性对账（Reconcile）。

本质是把 "一堆独立消费组" 封装成一个可整体启停、可健康探测的运行时单元，方便挂到应用主流程里。每个读模型独立消费组的配置、Handler、Lag 监控器都在 hgConsumerWorker 里封装，HGRuntime 负责统一管理。
*/

const hgConsumerShutdownTimeout = 10 * time.Second

type hgConsumerWorker struct {
	name        string // 消费组标识：feed/statistic/...
	config      HGKafkaPackage.HGConsumerConfig
	handler     consumer.Handler                      // 业务消息处理器
	implemented bool                                  // 该消费组是否已实现（未实现的禁用）
	client      *kgo.Client                           // kgo 客户端（franz-go 库）
	lagObserver *HGKafkaPackage.HGConsumerLagObserver // 消费延迟监控
}

// RuntimeDependencies 是 Kafka 运行期依赖注入结构体，包含各读模型所需的外部依赖。
// 把各读模型需要的外部依赖（Redis、ClickHouse、MySQL、Projector 等）集中注入，Redis 复用应用长连接池，避免每个 Consumer 各自建连接池导致资源膨胀。
type RuntimeDependencies struct {
	Redis                      FeedConsumerPackage.RedisEvalClient
	FeedShardCount             int
	FeedMaxItems               int
	FeedGeneration             string
	StatisticStore             StatisticConsumerPackage.EventStore
	StatisticAggregate         StatisticConsumerPackage.AggregateReader
	StatisticRedis             StatisticConsumerPackage.RedisHashReader
	StatisticConfig            StatisticConsumerPackage.HGProjectionConfig
	StatisticReconcileConfig   StatisticConsumerPackage.HGReconcileConfig
	StatisticReconcileInterval time.Duration
	InteractionStore           InteractionConsumerPackage.EventStore
	DanmakuStore               DanmakuConsumerPackage.HistoryStore
	DanmakuRecent              DanmakuConsumerPackage.RecentProjector
}

// HGRuntime 管理各读模型独立消费组的创建、运行与关闭。
type HGRuntime struct {
	ctx               context.Context //全局上下文，cancel() 触发所有 worker 退出
	cancel            context.CancelFunc
	workers           []hgConsumerWorker //所有已启用的消费组
	wg                sync.WaitGroup     //等待所有 goroutine 退出
	once              sync.Once          //保证 Close() 幂等，多次调用只执行一次
	errMu             sync.RWMutex       //记录第一个出错的 worker，带读写锁
	err               error              //记录第一个出错的 worker，带读写锁
	workerMu          sync.RWMutex       //记录每个 worker 启动 / 停止状态，带锁保护（这里用了 map+Mutex，不是 sync.Map）f f
	workerStarted     map[string]bool
	workerStopped     map[string]error
	reconciler        *StatisticConsumerPackage.HGReconciler //统计读模型的周期性对账器
	reconcileInterval time.Duration
}

// NewRuntime 创建已启用的消费组；未实现消费者默认禁用，不会创建连接。
// 依赖前置校验：哪个消费组启用了，就必须提供对应依赖，否则直接 Close() 并返回 error。
//
//	@param parent
//	@param cfg
//	@param deps
//	@return *HGRuntime
//	@return error
func NewRuntime(parent context.Context, cfg HGKafkaPackage.HGClusterConfig, deps RuntimeDependencies) (*HGRuntime, error) {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	runtime := &HGRuntime{ctx: ctx, cancel: cancel, workerStarted: make(map[string]bool), workerStopped: make(map[string]error)}
	if (cfg.Consumers.Feed.Enabled || cfg.Consumers.Statistic.Enabled) && deps.Redis == nil {
		runtime.Close()
		return nil, fmt.Errorf("redis-backed kafka consumer dependency cannot be nil")
	}
	if cfg.Consumers.Statistic.Enabled && deps.StatisticStore == nil {
		runtime.Close()
		return nil, fmt.Errorf("Statistic ClickHouse authority store dependency cannot be nil")
	}
	if cfg.Consumers.Interaction.Enabled && deps.InteractionStore == nil {
		runtime.Close()
		return nil, fmt.Errorf("interaction mysql store dependency cannot be nil")
	}
	if cfg.Consumers.Danmaku.Enabled && (deps.DanmakuStore == nil || deps.DanmakuRecent == nil) {
		runtime.Close()
		return nil, fmt.Errorf("danmaku history store and recent projector cannot be nil")
	}

	// specs 声明所有消费组 spec：6 个读模型，其中 search 和 audit 的 implemented 默认为 false（占位，未实现）。
	specs := []hgConsumerWorker{
		{
			name:        "feed",
			config:      cfg.Consumers.Feed,
			handler:     FeedConsumerPackage.NewConsumer(FeedConsumerPackage.NewRedisProjector(deps.Redis, deps.FeedShardCount, deps.FeedMaxItems, deps.FeedGeneration)),
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
		{name: "interaction", config: cfg.Consumers.Interaction, handler: InteractionConsumerPackage.NewConsumer(deps.InteractionStore), implemented: true},
		{name: "danmaku", config: cfg.Consumers.Danmaku, handler: DanmakuConsumerPackage.NewConsumer(deps.DanmakuStore, deps.DanmakuRecent), implemented: true},
	}

	// for 遍历创建实际 worker
	for _, spec := range specs {
		// if 未启用的跳过
		if !spec.config.Enabled {
			continue
		}
		// if 启用但未实现的 → 直接报错（防止配置开了但代码没写）
		if !spec.implemented {
			runtime.Close()
			return nil, fmt.Errorf("%s kafka consumer is enabled but handler is not implemented", spec.name)
		}
		topics := spec.config.Topics
		if len(topics) == 0 {
			topics = cfg.Topics
		}
		opts, err := HGKafkaPackage.HGNewBusinessConsumerOpts(cfg, topics, spec.config.GroupID, spec.config.ClientID)
		if err != nil {
			runtime.Close()
			return nil, fmt.Errorf("build %s kafka consumer: %w", spec.name, err)
		}
		spec.lagObserver = HGKafkaPackage.HGNewConsumerLagObserver(spec.config.GroupID, topics)
		opts = append(opts, HGKafkaPackage.HGConsumerLagObserverOpts(spec.lagObserver)...)

		// 构建 kgo 客户端选项 → 创建 LagObserver → 把 LagObserver 的 hook 挂到 opts 里 → kgo.NewClient 创建客户端
		spec.client, err = kgo.NewClient(opts...)
		// 任何一步失败都调用 runtime.Close() 清理已创建的资源，避免泄漏
		if err != nil {
			spec.lagObserver.Close()
			runtime.Close()
			return nil, fmt.Errorf("new %s kafka consumer: %w", spec.name, err)
		}
		runtime.workers = append(runtime.workers, spec)
	}
	// if 可选初始化 Reconciler：Statistic 启用 + 依赖齐全 + 间隔 > 0 才创建。
	if cfg.Consumers.Statistic.Enabled && deps.StatisticAggregate != nil && deps.StatisticRedis != nil && deps.StatisticReconcileInterval > 0 {
		runtime.reconciler = StatisticConsumerPackage.NewHGReconciler(deps.StatisticAggregate, deps.StatisticRedis, deps.StatisticReconcileConfig)
		runtime.reconcileInterval = deps.StatisticReconcileInterval
	}
	// 设计亮点：失败时统一调 runtime.Close()，因为 Close 用 once 保护且会 cancel ctx，能把前面已经创建成功的 worker/client 都清理掉。
	return runtime, nil
}

// Start 启动所有已启用消费组。
func (r *HGRuntime) Start() {
	if r == nil {
		return
	}
	// 每个 worker 起一个 goroutine，调用 RunDomainEventConsumerWithLagObserver 跑消费循环
	for i := range r.workers {
		worker := &r.workers[i]
		r.wg.Add(1)
		go func() {
			defer r.wg.Done()
			// r worker 启动时标记 hgMarkWorkerStarted
			r.hgMarkWorkerStarted(worker.name)
			err := RunDomainEventConsumerWithLagObserver(r.ctx, worker.client, "business", worker.lagObserver, worker.handler)
			// if 只记录第一个错误（if r.err == nil），然后 r.cancel() 通知所有其他 worker 一起退出
			if r.ctx.Err() == nil { //worker 异常退出时：标记 hgMarkWorkerStopped
				r.hgMarkWorkerStopped(worker.name, err)
			}
			// 注意判断 r.ctx.Err() == nil：主动 cancel 导致的退出不记错误，区分 "异常崩溃" 和 "正常关闭"
			if err != nil && r.ctx.Err() == nil {
				r.errMu.Lock()
				if r.err == nil {
					r.err = fmt.Errorf("%s kafka consumer stopped: %w", worker.name, err)
				}
				r.errMu.Unlock()
				// 然后 r.cancel() 通知所有其他 worker 一起退出
				r.cancel()
			}
		}()
	}

	// 这类设计很常见：事件驱动负责实时更新，Ticker reconcile 负责周期性校准状态。
	// 如果有 reconciler，再起一个 goroutine 跑 time.Ticker 周期循环
	if r.reconciler != nil {
		r.wg.Add(1)
		go func() {
			defer r.wg.Done()

			// ticker 创建一个定时器，每隔 r.reconcileInterval 时间触发一次，用于周期性执行某个任务。
			ticker := time.NewTicker(r.reconcileInterval)
			defer ticker.Stop()
			for {
				select {
				case <-r.ctx.Done():
					return // 关闭信号
				case <-ticker.C: // 周期对账
					// 检查实际 partition
					// ├── 检查 offset
					// ├── 检查 lag
					// └── 修正 Observer 状态
					_, _ = r.reconciler.Reconcile(r.ctx)
				}
			}
		}()
	}
}

// 健康检查接口：Ready 返回消费 runtime 是否仍在运行。
// 给 k8s liveness/readiness 或应用主流程用，依次检查：
func (r *HGRuntime) Ready() error {
	// 全局 err 是否有值
	if r == nil {
		return nil
	}
	r.errMu.RLock()
	runtimeErr := r.err
	r.errMu.RUnlock()
	if runtimeErr != nil {
		return runtimeErr
	}
	r.workerMu.RLock()
	defer r.workerMu.RUnlock()
	for _, worker := range r.workers {
		// 每个 worker 是否已停止（workerStopped 有值就报错）
		if stoppedErr, stopped := r.workerStopped[worker.name]; stopped {
			return fmt.Errorf("%s kafka consumer stopped: %v", worker.name, stoppedErr)
		}
		// 每个 worker 是否已启动（workerStarted 为 false 就报错），全部通过返回 nil，否则返回具体哪个 worker 有问题。
		if !r.workerStarted[worker.name] {
			return fmt.Errorf("%s kafka consumer not started", worker.name)
		}
	}
	return nil
}

func (r *HGRuntime) hgMarkWorkerStarted(name string) {
	r.workerMu.Lock()
	defer r.workerMu.Unlock()
	if r.workerStarted == nil {
		r.workerStarted = make(map[string]bool)
	}
	r.workerStarted[name] = true
}

func (r *HGRuntime) hgMarkWorkerStopped(name string, err error) {
	r.workerMu.Lock()
	defer r.workerMu.Unlock()
	if r.workerStopped == nil {
		r.workerStopped = make(map[string]error)
	}
	if err == nil {
		err = fmt.Errorf("worker exited unexpectedly")
	}
	r.workerStopped[name] = err
}

// 优雅关闭：Close 停止并等待消费者，随后关闭各自 Kafka Client。
func (r *HGRuntime) Close() {
	if r == nil {
		return
	}
	// 用 wg.Wait() + select + time.After 做超时兜底（10 秒），防止某个 worker 卡住导致整个 Close 阻塞
	r.once.Do(func() {
		r.cancel() // 1. 发关闭信号，所有worker开始退出
		done := make(chan struct{})
		go func() { r.wg.Wait(); close(done) }() // 2. 异步等待所有goroutine退出
		select {
		case <-done: // 3a. 正常退出
		case <-time.After(hgConsumerShutdownTimeout): // 3b. 超时10秒强制继续
		}
		for i := range r.workers { // 4. 关闭LagObserver和Kafka Client
			r.workers[i].lagObserver.Close()
			r.workers[i].client.Close()
		}
	})
}
