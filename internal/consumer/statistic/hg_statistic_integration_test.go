package statistic_test

import (
	StatisticConsumerPackage "MLC_GO/internal/consumer/statistic"
	"MLC_GO/internal/events"
	VideoEventsPackage "MLC_GO/internal/events/video"
	InfrastructureKafkaPackage "MLC_GO/internal/infrastructure/kafka"
	ClickHousePackage "MLC_GO/internal/pkg/clickhouse"
	HGKafkaPackage "MLC_GO/internal/pkg/kafka"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
)

func TestHGStatisticIntegrationKafkaClickHouseRedisReconcile(t *testing.T) {
	if os.Getenv("MLC_STATISTIC_INTEGRATION") != "1" {
		t.Skip("set MLC_STATISTIC_INTEGRATION=1 to run Statistic integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	brokers := strings.Split(hgStatisticEnvOrDefault("MLC_KAFKA_BROKERS", "localhost:19092,localhost:29092,localhost:39092"), ",")
	// 每轮使用独立 topic、consumer group 和 Redis generation，避免历史 offset 或累计值污染验收结果。
	topic := fmt.Sprintf("mlc.statistic.acceptance.%d", time.Now().UnixNano())
	groupID := fmt.Sprintf("mlc-statistic-acceptance-%d", time.Now().UnixNano())
	generation := fmt.Sprintf("acceptance-%d", time.Now().UnixNano())
	hgCreateStatisticIntegrationTopic(t, ctx, brokers, topic)

	redisClient := redis.NewClient(&redis.Options{Addr: hgStatisticEnvOrDefault("MLC_STATISTIC_REDIS_ADDR", "127.0.0.1:16379")})
	defer redisClient.Close()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		t.Fatalf("ping Redis: %v", err)
	}
	redisService := hgStatisticRedisClient{client: redisClient}

	clickHouseClient, err := ClickHousePackage.NewHGClient(ClickHousePackage.HGConfig{
		Endpoint:             hgStatisticEnvOrDefault("MLC_STATISTIC_CLICKHOUSE_ENDPOINT", "http://127.0.0.1:18123"),
		Database:             "mlc",
		Username:             hgStatisticEnvOrDefault("MLC_STATISTIC_CLICKHOUSE_USER", "default"),
		Password:             os.Getenv("MLC_STATISTIC_CLICKHOUSE_PASSWORD"),
		StatisticEventsTable: "statistic_events",
		StatisticTotalsTable: "statistic_event_totals",
		WriteTimeout:         10 * time.Second,
		QueryTimeout:         10 * time.Second,
	})
	if err != nil {
		t.Fatalf("new ClickHouse client: %v", err)
	}
	defer clickHouseClient.Close()
	if err := clickHouseClient.PingContext(ctx); err != nil {
		t.Fatalf("ping ClickHouse: %v", err)
	}

	consumerOpts, err := HGKafkaPackage.HGNewBusinessConsumerOpts(HGKafkaPackage.HGClusterConfig{Brokers: brokers}, []string{topic}, groupID, groupID)
	if err != nil {
		t.Fatalf("build consumer options: %v", err)
	}
	kafkaConsumer, err := kgo.NewClient(consumerOpts...)
	if err != nil {
		t.Fatalf("new Kafka consumer: %v", err)
	}
	defer kafkaConsumer.Close()

	// 复用生产 Handler，真实验证 ClickHouse authority -> Redis projection -> Kafka commit 的执行顺序。
	handler := StatisticConsumerPackage.NewConsumer(StatisticConsumerPackage.NewRedisCounter(redisService, 64, generation), clickHouseClient, StatisticConsumerPackage.HGProjectionConfig{RedisGeneration: generation, RedisShardCount: 64})
	consumeCtx, stopConsumer := context.WithCancel(ctx)
	consumerDone := make(chan error, 1)
	go func() {
		consumerDone <- InfrastructureKafkaPackage.RunDomainEventConsumer(consumeCtx, kafkaConsumer, topic+".dlq", handler)
	}()

	event := VideoEventsPackage.VideoPublishedEvent{
		EventMeta:    events.NewEventMeta(ctx),
		SubmissionID: "acceptance-submission",
		UserID:       "acceptance-user",
	}
	envelope, err := events.NewEnvelope(event)
	if err != nil {
		t.Fatalf("build event envelope: %v", err)
	}
	payload, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("marshal event envelope: %v", err)
	}
	producer, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	if err != nil {
		t.Fatalf("new Kafka producer: %v", err)
	}
	defer producer.Close()
	if err := producer.ProduceSync(ctx, &kgo.Record{Topic: topic, Key: []byte(event.EventKey()), Value: payload}).FirstErr(); err != nil {
		t.Fatalf("produce statistic event: %v", err)
	}

	var totals map[ClickHousePackage.HGStatisticDimension]uint64
	var redisValue string
	for ctx.Err() == nil {
		totals, err = clickHouseClient.GetStatisticTotals(ctx, generation)
		if err != nil {
			t.Fatalf("read ClickHouse totals: %v", err)
		}
		redisValue, err = redisClient.HGet(ctx, PersistenceRedisPackage.GetVideoEventCounterKey(generation, 0), VideoEventsPackage.VideoPublishedEventName).Result()
		if err == nil && totals[ClickHousePackage.HGStatisticDimension{Shard: 0, EventName: VideoEventsPackage.VideoPublishedEventName}] == 1 && redisValue == "1" {
			break
		}
		if err != nil && err != redis.Nil {
			t.Fatalf("read Redis total: %v", err)
		}
		time.Sleep(200 * time.Millisecond)
	}
	if ctx.Err() != nil {
		t.Fatalf("wait statistic projection: %v", ctx.Err())
	}

	reconciler := StatisticConsumerPackage.NewHGReconciler(clickHouseClient, redisService, StatisticConsumerPackage.HGReconcileConfig{Generation: generation, ShardCount: 64, Timeout: 10 * time.Second})
	result, err := reconciler.Reconcile(ctx)
	if err != nil {
		t.Fatalf("reconcile Statistic projection: %v", err)
	}
	if result.MismatchedDimensions != 0 || result.AbsoluteDrift != 0 {
		t.Fatalf("reconciliation result = %+v, want zero drift", result)
	}

	// 先停止消费循环再读取 committed offset，避免组会话仍在处理提交造成不稳定断言。
	stopConsumer()
	select {
	case err := <-consumerDone:
		if err != nil && consumeCtx.Err() == nil {
			t.Fatalf("stop Statistic consumer: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Statistic consumer did not stop within 10s")
	}
	committed := kafkaConsumer.CommittedOffsets()[topic]
	if len(committed) == 0 || committed[0].Offset < 1 {
		t.Fatalf("committed offsets = %+v, want partition 0 offset at least 1", committed)
	}
}

func hgCreateStatisticIntegrationTopic(t *testing.T, ctx context.Context, brokers []string, topic string) {
	t.Helper()
	client, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	if err != nil {
		t.Fatalf("new Kafka admin client: %v", err)
	}
	defer client.Close()
	request := kmsg.NewPtrCreateTopicsRequest()
	request.Topics = append(request.Topics, kmsg.CreateTopicsRequestTopic{Topic: topic, NumPartitions: 1, ReplicationFactor: 3})
	response, err := request.RequestWith(ctx, client)
	if err != nil {
		t.Fatalf("create Statistic integration topic: %v", err)
	}
	if len(response.Topics) != 1 {
		t.Fatalf("create topic response count = %d, want 1", len(response.Topics))
	}
	if topicErr := kerr.ErrorForCode(response.Topics[0].ErrorCode); topicErr != nil && topicErr != kerr.TopicAlreadyExists {
		t.Fatalf("create Statistic integration topic: %v", topicErr)
	}
	// Controller 创建成功后元数据仍需传播到 broker，短暂等待避免首次 produce 命中 unknown topic。
	time.Sleep(500 * time.Millisecond)
}

func hgStatisticEnvOrDefault(key string, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

type hgStatisticRedisClient struct {
	client *redis.Client
}

// Eval 和 HGetAll 仅适配生产 Counter/Reconciler 所需的最小 Redis 接口，避免测试依赖全局 Redis 客户端。
func (c hgStatisticRedisClient) Eval(ctx context.Context, script string, keys []string, args ...any) error {
	return c.client.Eval(ctx, script, keys, args...).Err()
}

func (c hgStatisticRedisClient) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return c.client.HGetAll(ctx, key).Result()
}
