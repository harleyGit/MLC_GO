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
	name        string                                // Worker 名称；消费组标识：feed/statistic/...
	config      HGKafkaPackage.HGConsumerConfig       // Kafka 消费者配置
	handler     consumer.Handler                      // 处理 Kafka 消息的业务对象
	implemented bool                                  // 该消费组是否已实现（未实现的禁用）
	client      *kgo.Client                           // 后续创建的 Kafka 客户端；kgo 客户端（franz-go 库）
	lagObserver *HGKafkaPackage.HGConsumerLagObserver // 消费积压监控对象；消费延迟监控
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

// 根据配置初始化 Kafka 消费者运行环境，为不同业务创建独立的 Kafka Consumer，并统一管理它们的生命周期。
// NewRuntime() 相当于 Kafka 消费系统的初始化管理器。它根据配置选择消费者、检查依赖、创建 Kafka Client、安装监控组件，并将所有 Worker 交给 HGRuntime 统一管理。
// 例如：Feed、Statistic、Interaction、Danmaku 等消费者。
// 需要特别注意：这个方法主要负责初始化，不代表已经开始消费 Kafka 消息。 真正启动消费通常还需要调用 Start()、Run() 等方法。
// 依赖前置校验：哪个消费组启用了，就必须提供对应依赖，否则直接 Close() 并返回 error。
//
//	@param parent
//	@param cfg
//	@param deps
//	@return *HGRuntime
//	@return error
/* 整体执行流程
NewRuntime(parent, cfg, deps)
        |
        v
1. 创建 Context 和 Cancel
        |
        v
2. 初始化 HGRuntime
        |
        v
3. 检查依赖
   Redis / MySQL / ClickHouse 等
        |
        v
4. 定义 6 个 Consumer Worker
   |
   +-- feed         已实现
   +-- search       未实现
   +-- statistic    已实现
   +-- audit        未实现
   +-- interaction  已实现
   +-- danmaku      已实现
        |
        v
5. 遍历 Worker
   |
   +-- 未启用 → 跳过
   |
   +-- 未实现 → 报错
   |
   +-- 已启用且已实现
          |
          +-- 获取 Topics
          +-- 构建 Kafka 配置
          +-- 创建 LagObserver
          +-- 创建 kgo.Client
          +-- 加入 runtime.workers
        |
        v
6. 可选创建 Statistic Reconciler
        |
        v
7. 返回 HGRuntime
*/
func NewRuntime(parent context.Context, cfg HGKafkaPackage.HGClusterConfig, deps RuntimeDependencies) (*HGRuntime, error) {

	// 1. 创建 Context：统一控制生命周期
	if parent == nil {
		parent = context.Background()
	}
	// ctx：上下文，用于传递取消信号。
	// cancel：取消函数，调用后可以通知使用该 ctx 的任务停止。
	// 调用 cancel() 后，监听 ctx.Done() 的消费者循环就可以收到退出信号
	ctx, cancel := context.WithCancel(parent)

	// 创建Runtime
	// 记录 Worker 是否启动
	// 记录 Worker 停止时的错误信息
	runtime := &HGRuntime{ctx: ctx, cancel: cancel, workerStarted: make(map[string]bool), workerStopped: make(map[string]error)}
	if (cfg.Consumers.Feed.Enabled || cfg.Consumers.Statistic.Enabled) && deps.Redis == nil {
		runtime.Close()
		return nil, fmt.Errorf("redis-backed kafka consumer dependency cannot be nil")
	}
	if cfg.Consumers.Statistic.Enabled && deps.StatisticStore == nil {
		runtime.Close()
		return nil, fmt.Errorf("Statistic ClickHouse authority store dependency cannot be nil")
	}
	// 2. 检查依赖：防止运行时出现 nil 错误。意思是：如果 Interaction 消费者已经启用，那么必须提供 MySQL 存储依赖。
	if cfg.Consumers.Interaction.Enabled && deps.InteractionStore == nil {
		runtime.Close()
		return nil, fmt.Errorf("interaction mysql store dependency cannot be nil")
	}
	if cfg.Consumers.Danmaku.Enabled && (deps.DanmakuStore == nil || deps.DanmakuRecent == nil) {
		runtime.Close()
		return nil, fmt.Errorf("danmaku history store and recent projector cannot be nil")
	}

	// specs 声明所有消费组 spec：6 个读模型，其中 search 和 audit 的 implemented 默认为 false（占位，未实现）。
	// 3.specs: 定义所有消费者；specs 是一个切片，用来统一描述每个 Kafka Worker。
	// 可以把每个 hgConsumerWorker 理解成一份消费者配置说明书。
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
		// InteractionConsumerPackage.NewConsumer 它是创建一个 Interaction 业务处理器，负责处理点赞、关注等事件。
		{name: "interaction", config: cfg.Consumers.Interaction, handler: InteractionConsumerPackage.NewConsumer(deps.InteractionStore), implemented: true},
		{name: "danmaku", config: cfg.Consumers.Danmaku, handler: DanmakuConsumerPackage.NewConsumer(deps.DanmakuStore, deps.DanmakuRecent), implemented: true},
	}

	// for 遍历创建实际 worker; 4.遍历 specs：只创建需要的 Consumer
	for _, spec := range specs {
		// if 未启用的跳过
		if !spec.config.Enabled {
			/* 示例：
			cfg.Consumers.Feed.Enabled = true
			cfg.Consumers.Interaction.Enabled = true
			cfg.Consumers.Danmaku.Enabled = false

			那么只会创建 Feed 和 Interaction 对应的 Kafka 客户端，Danmaku 会被跳过。
			*/
			continue
		}
		// if 启用但未实现的 → 直接报错（防止配置开了但代码没写）
		if !spec.implemented {
			// 若是配置为启用，但代码还没有实现，就立即报错。
			// 这是一种 Fail Fast（快速失败） 设计，避免服务看似正常启动，实际却没有处理对应消息。
			runtime.Close()
			return nil, fmt.Errorf("%s kafka consumer is enabled but handler is not implemented", spec.name)
		}
		// 5.创建 Kafka Consumer 配置，意思是：优先使用当前 Consumer 自己的 Topics，如果没有配置，就使用全局 Topics。
		topics := spec.config.Topics
		if len(topics) == 0 {
			topics = cfg.Topics
		}
		// 创建 Kafka Consumer 所需的配置选项。
		// 这里两个概念需要区分：
		// - GroupID：消费者组 ID。同组 Consumer 可以协同分配 Topic 的分区。
		// - ClientID：Kafka 客户端标识，主要用于日志、监控和请求识别。
		opts, err := HGKafkaPackage.HGNewBusinessConsumerOpts(cfg, topics, spec.config.GroupID, spec.config.ClientID)
		if err != nil {
			runtime.Close()
			return nil, fmt.Errorf("build %s kafka consumer: %w", spec.name, err)
		}
		// 6.创建 LagObserver：监控 Kafka 消息积压，这里的 Lag 指 Kafka 消费积压量。
		/*
			Kafka 分区最新 Offset：10000
			Consumer 已提交 Offset：8000

			Lag ≈ 2000 条消息

			说明消费者大约落后了 2000 条消息。
			HGNewConsumerLagObserver() 创建监控对象，而 HGConsumerLagObserverOpts() 将它的监控 Hook 加入 Kafka 客户端选项。
			具体监控哪些指标，需要看这两个方法的实现。
		*/
		spec.lagObserver = HGKafkaPackage.HGNewConsumerLagObserver(spec.config.GroupID, topics)
		opts = append(opts, HGKafkaPackage.HGConsumerLagObserverOpts(spec.lagObserver)...)

		// 7. kgo.NewClient()：真正创建 Kafka 客户端，负责与 Kafka 集群通信、拉取消息等。
		// 构建 kgo 客户端选项 → 创建 LagObserver → 把 LagObserver 的 hook 挂到 opts 里 → kgo.NewClient 创建客户端
		spec.client, err = kgo.NewClient(opts...)
		// 任何一步失败都调用 runtime.Close() 清理已创建的资源，避免泄漏
		if err != nil {
			// 创建失败，清理已创建的资源
			spec.lagObserver.Close()
			runtime.Close()
			return nil, fmt.Errorf("new %s kafka consumer: %w", spec.name, err)
		}
		// 将 Worker 加入 Runtime，后续由 Runtime 统一启动和关闭。
		runtime.workers = append(runtime.workers, spec)
	}
	// 8. 创建 Reconciler：数据对账与修复
	// 可选初始化 Reconciler：Statistic 启用 + 依赖齐全 + 间隔 > 0 才创建。
	if cfg.Consumers.Statistic.Enabled && deps.StatisticAggregate != nil && deps.StatisticRedis != nil && deps.StatisticReconcileInterval > 0 {
		// 这里的 Reconciler 可以理解为数据对账器。
		/*举例：
		Redis 点赞计数：100
		ClickHouse 统计结果：105


		如果 ClickHouse 被设计为权威数据源，那么 Reconciler 可以根据既定规则检测差异并进行修复。
		不过实际以哪个存储为准、如何修复，需要查看 NewHGReconciler() 的具体实现。
		这里还需要注意：当前代码只是创建 Reconciler 对象，尚未看到它的执行或定时调度逻辑。
		*/
		runtime.reconciler = StatisticConsumerPackage.NewHGReconciler(deps.StatisticAggregate, deps.StatisticRedis, deps.StatisticReconcileConfig)
		runtime.reconcileInterval = deps.StatisticReconcileInterval
	}
	// 设计亮点：失败时统一调 runtime.Close()，因为 Close 用 once 保护且会 cancel ctx，能把前面已经创建成功的 worker/client 都清理掉。
	return runtime, nil
}

// Start 启动所有消费者，让它们并发处理消息
/*执行流程：
Start() 启动
       |
       +-- goroutine 1：Feed Consumer
       |
       +-- goroutine 2：Statistic Consumer
       |
       +-- goroutine 3：Interaction Consumer
       |
       +-- goroutine 4：Danmaku Consumer
       |
       +-- goroutine 5：Reconciler（可选）
       |
       v
       消费者并发运行
*/
func (r *HGRuntime) Start() {
	if r == nil {
		return
	}
	// 1.为每个 Worker 创建 goroutine，调用 RunDomainEventConsumerWithLagObserver 跑消费循环
	for i := range r.workers {
		worker := &r.workers[i]
		r.wg.Add(1) //告诉 WaitGroup 增加一个待完成任务
		go func() { //创建 goroutine，并发运行消费者
			defer r.wg.Done() //goroutine 退出时，待完成任务数减 1
			// 2. 记录 Worker 已启动
			r.hgMarkWorkerStarted(worker.name)
			err := RunDomainEventConsumerWithLagObserver(r.ctx, worker.client, "business", worker.lagObserver, worker.handler)
			// 3. Worker 异常退出时怎么办？
			// r.ctx.Err() 用来检查 Context 是否已经取消。
			// 	- nil：Runtime 尚未收到取消信号。
			// 	- 非 nil：Runtime 已经取消或超时。
			// 因此，如果消费者在没有收到取消信号时退出，就记录为异常停止。
			if r.ctx.Err() == nil { //worker 异常退出时：标记 hgMarkWorkerStopped
				r.hgMarkWorkerStopped(worker.name, err)
			}
			// 注意判断 r.ctx.Err() == nil：主动 cancel 导致的退出不记错误，区分 "异常崩溃" 和 "正常关闭"
			/* 假设 Interaction Consumer 发生数据库错误：
			Interaction Consumer
			        |
			        v
			发生错误，退出消费循环
			        |
			        v
			记录第一个错误到 r.err
			        |
			        v
			调用 r.cancel()
			        |
			        +-- Feed 收到取消信号
			        +-- Statistic 收到取消信号
			        +-- Danmaku 收到取消信号

			一个需要注意的细节： 如果消费循环意外返回 nil，代码会记录 Worker 已停止，但不会执行 r.cancel()。因此其他 Worker 可能继续运行，不过 Ready() 可以发现该 Worker 已经停止。
			*/
			if err != nil && r.ctx.Err() == nil {
				r.errMu.Lock() // 这里使用 errMu.Lock() 是为了防止多个 goroutine 同时修改 r.err，产生数据竞争。
				if r.err == nil {
					r.err = fmt.Errorf("%s kafka consumer stopped: %w", worker.name, err)
				}
				r.errMu.Unlock()
				// 然后 r.cancel() 通知所有其他 worker 一起退出
				r.cancel()
			}
		}()
	}

	// Reconciler：定时执行对账任务
	// 这类设计很常见：事件驱动负责实时更新，Ticker reconcile 负责周期性校准状态。
	// 如果有 reconciler，再起一个 goroutine 跑 time.Ticker 周期循环
	if r.reconciler != nil {
		r.wg.Add(1)
		go func() {
			defer r.wg.Done()

			// 使用 time.Ticker 创建周期性定时器，每隔 r.reconcileInterval 时间触发一次，用于周期性执行某个任务。
			/*
				启动 Reconciler
					|
					v
				等待 30 秒
					|
					v
				Reconcile() 执行对账
					|
					v
				等待下一次 Tick
					|
					+-- Context 取消 → 退出
					|
					+-- 未取消 → 继续对账
			*/
			ticker := time.NewTicker(r.reconcileInterval)
			defer ticker.Stop()
			for {
				select {
				case <-r.ctx.Done():
					return // 关闭信号
				case <-ticker.C: // 执行对账
					_, _ = r.reconciler.Reconcile(r.ctx)
				}
			}
		}()
	}
}

// 检查消费者是否正常运行。
/*执行流程：
消费者并发运行
       |
       +-- Ready() 检查运行状态
       |
       +-- 某个 Worker 异常
       |       |
       |       +-- 记录错误
       |       +-- r.cancel()
       |       +-- 通知其他 Worker 退出
       |
       v
*/
// 注意：Ready() 目前只能检查程序内部记录的状态，不能保证 Kafka 连接正常、消息消费没有卡住。
// 给 k8s liveness/readiness 或应用主流程用，依次检查：
func (r *HGRuntime) Ready() error {
	// 全局 err 是否有值
	if r == nil {
		return nil
	}
	// 使用读锁保护 Map，避免其他 goroutine 同时写入时产生并发问题。
	r.errMu.RLock()
	runtimeErr := r.err
	r.errMu.RUnlock()
	if runtimeErr != nil { //首先检查 Runtime 是否记录了全局错误。
		return runtimeErr
	}
	r.workerMu.RLock()
	defer r.workerMu.RUnlock()
	// 逐个检查worker，Worker 未启动，返回错误
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

// 通知消费者停止，等待退出并释放资源
/* 执行流程：
Close()
       |
       +-- 发送取消信号
       +-- 等待 goroutine 退出
       +-- 关闭 LagObserver
       +-- 关闭 Kafka Client
*/
func (r *HGRuntime) Close() {
	if r == nil {
		return
	}

	// once.Do() 保证只执行一次
	// 用 wg.Wait() + select + time.After 做超时兜底（10 秒），防止某个 worker 卡住导致整个 Close 阻塞
	r.once.Do(func() {
		// 1. 发关闭信号，所有worker开始退出；所有使用 r.ctx 的任务都能感知取消。但 cancel() 不是强制杀死 goroutine，需要消费循环主动检查 Context 并退出。
		r.cancel()
		done := make(chan struct{})
		// 2. 异步等待所有goroutine退出； wg.Wait() 等待任务退出；
		// r.wg.Wait()：等待所有登记的 goroutine 执行 Done()。
		// close(done)：通知外部，所有任务已经退出。
		go func() { r.wg.Wait(); close(done) }()
		// select 设置超时
		/*假设超时时间为 10 秒。
		调用 Close()
		|
		v
		等待所有 Worker 退出
		|
		+-- 3 秒内全部退出
		|       |
		|       +-- 立即继续清理
		|
		+-- 超过 10 秒
			|
			+-- 不再等待
			+-- 继续清理 Kafka Client

		这里的 10 秒只是等待超时，并不是强制终止 goroutine。
		如果某个 Worker 卡死，仍可能继续运行。
		*/
		select {
		case <-done: // 3a. 正常退出
		case <-time.After(hgConsumerShutdownTimeout): // 3b. 超时10秒强制继续
		}
		// 释放 Kafka 资源
		for i := range r.workers { // 4. 关闭LagObserver和Kafka Client
			r.workers[i].lagObserver.Close()
			r.workers[i].client.Close()
		}
	})
}
