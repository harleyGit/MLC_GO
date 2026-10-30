package module

import (
	IMConsumer "MLC_GO/internal/consumer/im"
	KafkaRuntime "MLC_GO/internal/infrastructure/kafka"
	IM "MLC_GO/internal/modules/im"
	"MLC_GO/internal/modules/im/transport"
	Kafka "MLC_GO/internal/pkg/kafka"
	"context"
	"database/sql"
	"fmt"
	"github.com/twmb/franz-go/pkg/kgo"
	"io"
	"net/http"
	"sync"
	"time"
)

// HGRuntime 统一管理独立 IM 消费及有界后台工作；Start/Close 配对，Close 等待所有 I/O 响应取消。
type HGRuntime struct {
	gateway        *transport.HGCentrifugo      // 网关连接池。
	storage        *transport.HGSeaweedFS       // 私有对象连接池。
	reviewer       *transport.HGReviewer        // 审核服务连接池。
	livekit        *transport.HGLiveKit         // 通话控制连接池。
	realtime       *IM.HGRealtime               // 设备投递任务。
	media          *IM.HGPrivateMedia           // 可选审核任务。
	calls          *IM.HGCalls                  // 可选呼叫超时/清理任务。
	client         *kgo.Client                  // IM 独立 Kafka 消费组。
	observer       *Kafka.HGConsumerLagObserver // 复用现有低基数 Kafka lag 指标。
	cancel         context.CancelFunc           // 应用关闭信号。
	wg             sync.WaitGroup               // 所有启动任务的退出屏障。
	start          sync.Once                    // 防止重复启动。
	close          sync.Once                    // 防止重复释放资源。
	mu             sync.Mutex                   // 保护启动与故障状态。
	started        bool                         // 未启动时 Ready 失败。
	closed         bool                         // 与 Start 互斥，防止 Close 后启动后台任务。
	consumerErr    error                        // 消费循环异常退出，不自动清除。
	workerErrors   map[string]error             // 固定三个后台任务的最近错误，恢复后清除。
	workerFailures map[string]uint64            // 固定低基数任务失败计数。
}

// HGRegisterConfigured 创建默认关闭或显式配置的 IM 模块，不启动任务、不执行迁移。
func HGRegisterConfigured(db *sql.DB, legacyTopic string, cfg HGConfig, cluster Kafka.HGClusterConfig) (*HGRuntime, error) {
	if !cfg.Realtime {
		hgRegisterHandler(IM.HGNewHandler(IM.HGNewService(db, legacyTopic, nil)))
		return nil, nil
	}
	for _, topic := range cluster.Topics {
		if topic == cfg.Topic {
			return nil, fmt.Errorf("IM topic 必须与共享业务 topic 隔离")
		}
	}
	for _, group := range []Kafka.HGConsumerConfig{cluster.Consumers.Feed, cluster.Consumers.Search, cluster.Consumers.Statistic, cluster.Consumers.Audit, cluster.Consumers.Interaction, cluster.Consumers.Notification, cluster.Consumers.Danmaku, cluster.Consumers.CrawlerFetch, cluster.Consumers.CrawlerClean, cluster.Consumers.CrawlerStore, cluster.Consumers.CrawlerHistory} {
		if cfg.Group == group.GroupID {
			return nil, fmt.Errorf("IM 消费组不得与其他投影共用")
		}
	}
	r := &HGRuntime{workerErrors: make(map[string]error), workerFailures: make(map[string]uint64)}
	success := false
	defer func() {
		if !success {
			r.Close()
		}
	}()
	var err error
	r.gateway, err = transport.HGNewCentrifugo(cfg.centrifugoURL, cfg.centrifugoKey, cfg.centrifugoSecret)
	if err != nil {
		return nil, err
	}
	if cfg.Media {
		r.storage, err = transport.HGNewSeaweedFS(cfg.s3URL, cfg.s3Region, cfg.s3Bucket, cfg.s3Key, cfg.s3Secret)
		if err != nil {
			return nil, err
		}
		r.reviewer, err = transport.HGNewReviewer(cfg.reviewURL, cfg.reviewToken)
		if err != nil {
			return nil, err
		}
		r.media = IM.HGNewPrivateMedia(db, r.storage, r.reviewer, cfg.s3Bucket)
	}
	var mediaAuthorizer IM.HGMediaAuthorizer
	if r.media != nil {
		mediaAuthorizer = r.media
	}
	service := IM.HGNewService(db, cfg.Topic, mediaAuthorizer)
	if cfg.Calls {
		r.livekit, err = transport.HGNewLiveKit(cfg.livekitURL, cfg.livekitKey, cfg.livekitSecret)
		if err != nil {
			return nil, err
		}
		r.calls = IM.HGNewCalls(service, r.livekit, cfg.LiveKitURL)
	}
	r.realtime = IM.HGNewRealtime(db, r.gateway, cfg.Topic)
	opts, err := Kafka.HGNewBusinessConsumerOpts(cluster, []string{cfg.Topic}, cfg.Group, cfg.Group+"-worker")
	if err != nil {
		return nil, err
	}
	r.observer = Kafka.HGNewConsumerLagObserver(cfg.Group, []string{cfg.Topic})
	opts = append(opts, Kafka.HGConsumerLagObserverOpts(r.observer)...)
	r.client, err = kgo.NewClient(opts...)
	if err != nil {
		return nil, err
	}
	hgRegisterHandler(IM.HGNewFeatureHandler(service, r.gateway, r.media, r.calls, cfg.WebSocketURL))
	success = true
	return r, nil
}

func (r *HGRuntime) Start() {
	if r == nil {
		return
	}
	r.start.Do(func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		if r.closed {
			return
		}
		ctx, cancel := context.WithCancel(context.Background())
		r.cancel = cancel
		r.started = true
		r.wg.Add(1)
		go func() {
			defer r.wg.Done()
			err := KafkaRuntime.RunDomainEventConsumerWithLagObserver(ctx, r.client, "business", r.observer, IMConsumer.HGNewConsumer(r.realtime))
			if ctx.Err() == nil {
				r.mu.Lock()
				r.consumerErr = fmt.Errorf("IM 消费循环停止: %v", err)
				r.mu.Unlock()
			}
		}()
		r.hgWorker(ctx, "delivery", r.realtime.HGDispatchOnce)
		if r.media != nil {
			r.hgWorker(ctx, "media", r.media.HGReviewOnce)
		}
		if r.calls != nil {
			r.hgWorker(ctx, "calls", r.calls.HGMaintainOnce)
		}
	})
}

func (r *HGRuntime) hgWorker(ctx context.Context, name string, run func(context.Context) error) {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				work, stop := context.WithTimeout(ctx, 25*time.Second)
				err := run(work)
				stop()
				r.mu.Lock()
				r.workerErrors[name] = err
				if err != nil {
					r.workerFailures[name]++
				}
				r.mu.Unlock()
			}
		}
	}()
}

// Ready 报告任务生命周期与最近后台错误，不将此探针视作外部媒体全链路验收。
func (r *HGRuntime) Ready() error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.started {
		return fmt.Errorf("IM 后台尚未启动")
	}
	if r.closed {
		return fmt.Errorf("IM 后台已关闭")
	}
	if r.consumerErr != nil {
		return r.consumerErr
	}
	for _, name := range []string{"delivery", "media", "calls"} {
		if r.workerErrors[name] != nil {
			return fmt.Errorf("IM %s 后台异常", name)
		}
	}
	return nil
}

func (r *HGRuntime) HGWritePrometheusMetrics(w io.Writer) {
	if r == nil {
		return
	}
	r.realtime.HGWriteMetrics(w)
	r.mu.Lock()
	defer r.mu.Unlock()
	fmt.Fprintln(w, "# TYPE mlc_im_worker_failures_total counter")
	for _, name := range []string{"delivery", "media", "calls"} {
		fmt.Fprintf(w, "mlc_im_worker_failures_total{worker=%q} %d\n", name, r.workerFailures[name])
	}
}

// HGMountManagement 注册验签的服务器回调，仅允许 LiveKit 到管理端口的受控网络访问。
func (r *HGRuntime) HGMountManagement(mux *http.ServeMux) {
	if r != nil && r.calls != nil {
		mux.HandleFunc("POST /internal/im/livekit/webhook", r.calls.HGWebhookHandler)
	}
}

func (r *HGRuntime) Close() {
	if r == nil {
		return
	}
	r.close.Do(func() {
		r.mu.Lock()
		r.closed = true
		if r.cancel != nil {
			r.cancel()
		}
		r.mu.Unlock()
		r.wg.Wait()
		if r.client != nil {
			r.client.Close()
		}
		if r.observer != nil {
			r.observer.Close()
		}
		r.gateway.HGClose()
		r.storage.HGClose()
		r.reviewer.HGClose()
		r.livekit.HGClose()
	})
}
