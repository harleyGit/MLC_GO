/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-07-04 16:48:12
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-07-04 17:43:17
 * @FilePath: /MLC_GO/hg_kafka_application.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package main

import (
	DanmakuConsumerPackage "MLC_GO/internal/consumer/danmaku"
	StatisticConsumerPackage "MLC_GO/internal/consumer/statistic"
	InfrastructureEventBusPackage "MLC_GO/internal/infrastructure/eventbus"
	InfrastructureKafkaPackage "MLC_GO/internal/infrastructure/kafka"
	VideoInteractionRepositoryPackage "MLC_GO/internal/modules/video_interaction/repository"
	"MLC_GO/internal/outbox"
	ClickHousePackage "MLC_GO/internal/pkg/clickhouse"
	ConfigPackage "MLC_GO/internal/pkg/config"
	HGKafkaPackage "MLC_GO/internal/pkg/kafka"
	PersistenceSQLPackage "MLC_GO/internal/pkg/mysql"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"MLC_GO/internal/pkg/logHG"
)

// kafkaCloser 表示应用关闭阶段需要释放的 Kafka 资源。
type kafkaCloser func()

type kafkaReadyChecker interface {
	Ready() error
}

// initKafkaIfConfigured 按当前运行配置初始化 Kafka。
//
// Kafka 是本工程的启动必需基础设施，初始化过程分为三步：
// 1. GetKafkaConfig 读取并静态校验 kafka.business 配置；business.brokers 缺失或 acks 非法时立即返回错误。
// 2. HGInitKafka 创建 franz-go Client，并在启动阶段 Ping broker，避免服务启动后才发现网络或地址错误。
// 3. 初始化成功后返回 HGCloseKafka，由应用关闭流程负责 flush 缓冲消息并释放 broker 连接。
//
// pre/prod 任一步失败都会阻止 HTTP Server 启动。debug 默认允许 broker 未启动时降级运行，
// 便于本地调试非 Kafka 功能；设置 KAFKA_REQUIRED=true 可让 debug 使用与生产一致的快速失败策略。
//
// 返回 kafkaCloser 而不是在 main 中直接引用 HGCloseKafka，是为了让主应用只关心“是否有资源需要关闭”：
// 1. 非 nil 表示 Kafka 已完成初始化，Close 阶段必须 flush 并关闭 client。
// 2. 初始化失败不会返回 closer，调用方应按启动失败路径回滚前置依赖。
func initKafkaIfConfigured() (kafkaCloser, error) {
	closer, _, err := initKafkaWithDependencies(nil, nil)
	return closer, err
}

// initKafkaWithDependencies 初始化 Producer、独立 Consumer Group 和 Outbox dispatcher。
// Redis/MySQL 均复用应用现有连接池，避免后台任务重复创建高成本基础设施连接。
func initKafkaWithDependencies(redisService *PersistenceRedisPackage.RedisService, sqlManager *PersistenceSQLPackage.HGSQLManager) (kafkaCloser, kafkaReadyChecker, error) {
	producerCloser, err := hgInitKafkaIfConfigured(HGKafkaPackage.HGInitKafka)
	if err != nil || producerCloser == nil {
		return producerCloser, nil, err
	}
	cfg, _, err := ConfigPackage.GetKafkaConfig()
	if err != nil {
		producerCloser()
		return nil, nil, err
	}
	clickHouseConfig, statisticConfig, err := ConfigPackage.GetStatisticInfrastructureConfig()
	if err != nil {
		producerCloser()
		return nil, nil, fmt.Errorf("Statistic基础设施配置失败: %w", err)
	}
	var clickHouseClient *ClickHousePackage.HGClient
	if cfg.Business.Consumers.Statistic.Enabled || cfg.Business.Consumers.Danmaku.Enabled {
		if !clickHouseConfig.Enabled {
			producerCloser()
			return nil, nil, fmt.Errorf("Statistic消费者启用时 ClickHouse 必须启用")
		}
		clickHouseClient, err = ClickHousePackage.NewHGClient(ClickHousePackage.HGConfig{
			Endpoint: fmt.Sprintf("%s://%s:%s", clickHouseConfig.Scheme, clickHouseConfig.Host, clickHouseConfig.Port),
			Database: clickHouseConfig.Database, Username: clickHouseConfig.User, Password: clickHouseConfig.Password,
			StatisticEventsTable: clickHouseConfig.StatisticEventsTable, StatisticTotalsTable: clickHouseConfig.StatisticTotalsTable,
			DanmakuHistoryTable: clickHouseConfig.DanmakuHistoryTable,
			WriteTimeout:        clickHouseConfig.WriteTimeoutDuration, QueryTimeout: clickHouseConfig.QueryTimeoutDuration,
		})
		if err != nil {
			producerCloser()
			return nil, nil, fmt.Errorf("ClickHouse客户端初始化失败: %w", err)
		}
		pingCtx, cancel := context.WithTimeout(context.Background(), clickHouseConfig.QueryTimeoutDuration)
		err = clickHouseClient.PingContext(pingCtx)
		cancel()
		if err != nil {
			_ = clickHouseClient.Close()
			producerCloser()
			return nil, nil, fmt.Errorf("ClickHouse连接失败: %w", err)
		}
	}
	runtimeDeps := InfrastructureKafkaPackage.RuntimeDependencies{
		Redis: redisService, StatisticStore: clickHouseClient, StatisticAggregate: clickHouseClient, StatisticRedis: redisService,
		StatisticConfig: StatisticConsumerPackage.HGProjectionConfig{RedisGeneration: statisticConfig.RedisGeneration, RedisShardCount: statisticConfig.RedisShardCount},
	}
	videoRecommendConfig, err := ConfigPackage.GetVideoRecommendConfig()
	if err != nil {
		if clickHouseClient != nil {
			_ = clickHouseClient.Close()
		}
		producerCloser()
		return nil, nil, err
	}
	runtimeDeps.FeedGeneration = videoRecommendConfig.RedisGeneration
	runtimeDeps.FeedShardCount = videoRecommendConfig.RedisShardCount
	runtimeDeps.FeedMaxItems = videoRecommendConfig.RedisMaxItems
	if cfg.Business.Consumers.Danmaku.Enabled {
		if clickHouseConfig.DanmakuHistoryTable == "" {
			_ = clickHouseClient.Close()
			producerCloser()
			return nil, nil, fmt.Errorf("弹幕消费者启用时 ClickHouse danmaku_history_table 不能为空")
		}
		danmakuConfig, configErr := ConfigPackage.GetVideoDanmakuConfig()
		if configErr != nil {
			_ = clickHouseClient.Close()
			producerCloser()
			return nil, nil, configErr
		}
		runtimeDeps.DanmakuStore = clickHouseClient
		runtimeDeps.DanmakuRecent = DanmakuConsumerPackage.NewRecentProjector(redisService, danmakuConfig.RecentMessageLimit)
	}
	if sqlManager != nil {
		topic := ""
		if len(cfg.Business.Topics) > 0 {
			topic = cfg.Business.Topics[0]
		}
		runtimeDeps.InteractionStore = VideoInteractionRepositoryPackage.NewRepositoryWithTopic(sqlManager.GetSQLDB(), topic)
	}
	if statisticConfig.ReconcileEnabled {
		runtimeDeps.StatisticReconcileConfig = StatisticConsumerPackage.HGReconcileConfig{Generation: statisticConfig.RedisGeneration, ShardCount: statisticConfig.RedisShardCount, Timeout: statisticConfig.ReconcileTimeout}
		runtimeDeps.StatisticReconcileInterval = statisticConfig.ReconcileInterval
	}
	consumerRuntime, err := InfrastructureKafkaPackage.NewRuntime(context.Background(), cfg.Business, runtimeDeps)
	if err != nil {
		if clickHouseClient != nil {
			_ = clickHouseClient.Close()
		}
		producerCloser()
		return nil, nil, fmt.Errorf("Kafka消费者初始化失败: %w", err)
	}
	consumerRuntime.Start()

	// Outbox 与消费者共用一个可取消生命周期。先停 dispatcher，再停 consumer，最后 flush producer。
	dispatcherCtx, cancelDispatcher := context.WithCancel(context.Background())
	var dispatcherWG sync.WaitGroup
	if sqlManager != nil {
		topic := ""
		if len(cfg.Business.Topics) > 0 {
			topic = cfg.Business.Topics[0]
		}
		dispatcher := outbox.NewDispatcher(outbox.NewRepository(sqlManager.GetSQLDB(), topic), InfrastructureEventBusPackage.KafkaByteProducer{})
		dispatcherWG.Add(1)
		go func() {
			defer dispatcherWG.Done()
			if runErr := dispatcher.Run(dispatcherCtx, time.Second, 100); runErr != nil && dispatcherCtx.Err() == nil {
				logHG.ErrFInfo("Outbox dispatcher stopped: %v", runErr)
			}
		}()
	}
	return func() {
		cancelDispatcher()
		dispatcherWG.Wait()
		consumerRuntime.Close()
		if clickHouseClient != nil {
			_ = clickHouseClient.Close()
		}
		producerCloser()
	}, consumerRuntime, nil
}

func hgInitKafkaIfConfigured(initKafka func(HGKafkaPackage.HGKafkaClusterConfig) error) (kafkaCloser, error) {
	// GetKafkaConfig 只负责读取、清洗和静态校验配置，不发起网络请求。
	cfg, enabled, err := ConfigPackage.GetKafkaConfig()
	if err != nil {
		return nil, err
	}
	// business.brokers 已被定义为必填项，正常配置下 enabled 必须为 true。
	// 保留该检查作为防御边界，避免未来配置读取逻辑调整后意外跳过 Kafka 初始化。
	if !enabled {
		return nil, fmt.Errorf("Kafka配置未启用")
	}

	// HGInitKafka 会再次归一化 business 配置、创建全局 franz-go Client，并在固定超时内 Ping broker。
	// 只有 Ping 成功后 Client 才会发布到全局变量，因此失败初始化不会留下不可用的半成品 Client。
	if err := initKafka(cfg); err != nil {
		required, requiredErr := hgKafkaRequired()
		if requiredErr != nil {
			return nil, requiredErr
		}
		if required {
			return nil, fmt.Errorf("Kafka初始化失败: %w", err)
		}
		logHG.DebugFInfo("debug 环境 Kafka 未就绪，应用将以降级模式启动: %v", err)
		return nil, nil
	}

	// 该日志只表示 Kafka 已真实初始化并通过启动期 broker 探测，不会在缺少配置时输出误导信息。
	logHG.DebugInfo("Kafka初始化完成")
	return HGKafkaPackage.HGCloseKafka, nil
}

func hgKafkaRequired() (bool, error) {
	if ConfigPackage.GetEnv() != ConfigPackage.EnvDebug {
		return true, nil
	}

	value := os.Getenv("KAFKA_REQUIRED")
	if value == "" {
		return false, nil
	}
	required, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("KAFKA_REQUIRED 配置非法 %q: %w", value, err)
	}
	return required, nil
}
