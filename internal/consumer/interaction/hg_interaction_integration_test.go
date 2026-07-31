package interaction_test

import (
	InteractionConsumerPackage "MLC_GO/internal/consumer/interaction"
	"MLC_GO/internal/events"
	InteractionEventsPackage "MLC_GO/internal/events/interaction"
	InfrastructureKafkaPackage "MLC_GO/internal/infrastructure/kafka"
	VideoInteractionCachePackage "MLC_GO/internal/modules/video_interaction/cache"
	VideoInteractionRepositoryPackage "MLC_GO/internal/modules/video_interaction/repository"
	HGKafkaPackage "MLC_GO/internal/pkg/kafka"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/spf13/viper"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
)

func TestHGInteractionIntegrationKafkaMySQLRedisRecovery(t *testing.T) {
	if os.Getenv("MLC_INTERACTION_INTEGRATION") != "1" {
		t.Skip("set MLC_INTERACTION_INTEGRATION=1 to run Interaction integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	brokers := strings.Split(hgInteractionEnvOrDefault("MLC_KAFKA_BROKERS", "localhost:19092,localhost:29092,localhost:39092"), ",")
	topic := fmt.Sprintf("mlc.interaction.acceptance.%d", time.Now().UnixNano())
	groupID := fmt.Sprintf("mlc-interaction-acceptance-%d", time.Now().UnixNano())
	hgCreateInteractionTopic(t, ctx, brokers, topic)

	db, err := sql.Open("mysql", hgInteractionEnvOrDefault("MLC_INTERACTION_MYSQL_DSN", "root:hh109@tcp(127.0.0.1:3306)/HG_MLC_DB?parseTime=true&loc=UTC"))
	if err != nil {
		t.Fatalf("open MySQL: %v", err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping MySQL: %v", err)
	}

	viper.Set("redis.host", hgInteractionEnvOrDefault("MLC_INTERACTION_REDIS_HOST", "127.0.0.1"))
	viper.Set("redis.port", hgInteractionEnvOrDefault("MLC_INTERACTION_REDIS_PORT", "6379"))
	t.Cleanup(viper.Reset)
	redisService, err := PersistenceRedisPackage.NewRedisServiceWithError(ctx)
	if err != nil {
		t.Fatalf("connect Redis: %v", err)
	}
	defer redisService.Close()
	cache := VideoInteractionCachePackage.NewCache(redisService)

	producer, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	if err != nil {
		t.Fatalf("new Kafka producer: %v", err)
	}
	defer producer.Close()
	if err := HGKafkaPackage.HGInitKafka(HGKafkaPackage.HGKafkaClusterConfig{Business: HGKafkaPackage.HGClusterConfig{Brokers: brokers, Acks: "all"}, Log: HGKafkaPackage.HGClusterConfig{Brokers: brokers, Acks: "1"}}); err != nil {
		t.Fatalf("init Kafka DLQ producer: %v", err)
	}
	defer HGKafkaPackage.HGCloseKafka()

	userID := fmt.Sprintf("acceptance-user-%d", time.Now().UnixNano())
	submissionID := fmt.Sprintf("acceptance-submission-%d", time.Now().UnixNano())
	hgVerifyCoinTransaction(t, ctx, db, userID, submissionID)
	like := InteractionEventsPackage.VideoInteractionChangedEvent{EventMeta: events.NewEventMeta(ctx), ActorUserID: userID, SubmissionID: submissionID, Action: "like", Active: true}
	unlike := like
	unlike.Active = false
	likeRecord := hgInteractionRecord(t, topic, like)
	unlikeRecord := hgInteractionRecord(t, topic, unlike)

	stopFirst := hgStartInteractionConsumer(t, ctx, brokers, topic, groupID, db)
	if err := cache.ApplyOptimistic(ctx, userID, submissionID, "", "like", true, 0); err != nil {
		t.Fatalf("project optimistic like: %v", err)
	}
	if err := cache.ApplyOptimistic(ctx, userID, submissionID, "", "like", false, 0); err != nil {
		t.Fatalf("project optimistic unlike: %v", err)
	}
	if err := producer.ProduceSync(ctx, likeRecord, unlikeRecord).FirstErr(); err != nil {
		t.Fatalf("produce rapid like/unlike: %v", err)
	}
	hgAwaitInteractionState(t, ctx, db, userID, submissionID, false, 2)
	stopFirst()

	// 相同 consumer group 重启后重放同一 event_id，不得重复改变统计。
	secondConsumer, stopSecond := hgStartInteractionConsumerWithClient(t, ctx, brokers, topic, groupID, db)
	if err := producer.ProduceSync(ctx, likeRecord).FirstErr(); err != nil {
		t.Fatalf("produce duplicate interaction: %v", err)
	}
	hgAwaitCommittedOffset(t, ctx, secondConsumer, topic, 3)
	stopSecond()

	state, err := cache.GetState(ctx, userID, submissionID, "")
	if err != nil {
		t.Fatalf("read Redis interaction state: %v", err)
	}
	if state.Liked || state.LikeCount != 0 {
		t.Fatalf("Redis state = %+v, want rapid toggle final false and count 0", state)
	}

	closedDB, err := sql.Open("mysql", hgInteractionEnvOrDefault("MLC_INTERACTION_MYSQL_DSN", "root:hh109@tcp(127.0.0.1:3306)/HG_MLC_DB?parseTime=true&loc=UTC"))
	if err != nil {
		t.Fatal(err)
	}
	closedDB.Close()
	failureGroup := groupID + "-db-failure"
	consumer, stopFailure := hgStartInteractionConsumerWithClient(t, ctx, brokers, topic, failureGroup, closedDB)
	failureRecord := hgInteractionRecord(t, topic, InteractionEventsPackage.VideoInteractionChangedEvent{EventMeta: events.NewEventMeta(ctx), ActorUserID: userID, SubmissionID: submissionID + "-failure", Action: "like", Active: true})
	if err := producer.ProduceSync(ctx, failureRecord).FirstErr(); err != nil {
		t.Fatalf("produce DB failure event: %v", err)
	}
	time.Sleep(time.Second)
	stopFailure()
	for _, offset := range consumer.CommittedOffsets()[topic] {
		if offset.Offset > 0 {
			t.Fatalf("DB unavailable committed offset = %d, want no advancement", offset.Offset)
		}
	}
}

func hgVerifyCoinTransaction(t *testing.T, ctx context.Context, db *sql.DB, userID string, submissionID string) {
	t.Helper()
	if _, err := db.ExecContext(ctx, `INSERT INTO user_coin_wallets (user_id, balance) VALUES (?, 10) ON DUPLICATE KEY UPDATE balance = 10`, userID); err != nil {
		t.Fatalf("seed coin wallet: %v", err)
	}
	repo := VideoInteractionRepositoryPackage.NewRepository(db)
	event := InteractionEventsPackage.VideoInteractionChangedEvent{EventMeta: events.NewEventMeta(ctx), ActorUserID: userID, SubmissionID: submissionID, Action: "coin", Active: true, Quantity: 2}
	committed, err := repo.SubmitCoin(ctx, "acceptance-request-1", event)
	if err != nil || !committed {
		t.Fatalf("submit coin transaction committed=%t err=%v", committed, err)
	}
	committed, err = repo.SubmitCoin(ctx, "acceptance-request-1", event)
	if err != nil || committed {
		t.Fatalf("replay coin transaction committed=%t err=%v", committed, err)
	}
	var balance int
	var ledgerRows int
	var outboxRows int
	if err := db.QueryRowContext(ctx, `SELECT balance FROM user_coin_wallets WHERE user_id = ?`, userID).Scan(&balance); err != nil {
		t.Fatalf("read coin wallet: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_coin_ledger WHERE user_id = ? AND request_id = ?`, userID, "acceptance-request-1").Scan(&ledgerRows); err != nil {
		t.Fatalf("read coin ledger: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_events WHERE event_key = ? AND event_name = ?`, event.EventKey(), event.EventName()).Scan(&outboxRows); err != nil {
		t.Fatalf("read coin outbox: %v", err)
	}
	if balance != 8 || ledgerRows != 1 || outboxRows != 1 {
		t.Fatalf("coin transaction balance=%d ledger=%d outbox=%d", balance, ledgerRows, outboxRows)
	}
}

func hgAwaitCommittedOffset(t *testing.T, ctx context.Context, client *kgo.Client, topic string, want int64) {
	t.Helper()
	for ctx.Err() == nil {
		for _, offset := range client.CommittedOffsets()[topic] {
			if offset.Offset >= want {
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("wait committed offset %d: %v", want, ctx.Err())
}

func hgStartInteractionConsumer(t *testing.T, parent context.Context, brokers []string, topic string, groupID string, db *sql.DB) func() {
	t.Helper()
	_, stop := hgStartInteractionConsumerWithClient(t, parent, brokers, topic, groupID, db)
	return stop
}

func hgStartInteractionConsumerWithClient(t *testing.T, parent context.Context, brokers []string, topic string, groupID string, db *sql.DB) (*kgo.Client, func()) {
	t.Helper()
	opts, err := HGKafkaPackage.HGNewBusinessConsumerOpts(HGKafkaPackage.HGClusterConfig{Brokers: brokers}, []string{topic}, groupID, groupID)
	if err != nil {
		t.Fatalf("build consumer options: %v", err)
	}
	client, err := kgo.NewClient(opts...)
	if err != nil {
		t.Fatalf("new Kafka consumer: %v", err)
	}
	consumeCtx, cancel := context.WithCancel(parent)
	done := make(chan error, 1)
	handler := InteractionConsumerPackage.NewConsumer(VideoInteractionRepositoryPackage.NewRepository(db))
	go func() {
		done <- InfrastructureKafkaPackage.RunDomainEventConsumer(consumeCtx, client, "business", handler)
	}()
	return client, func() {
		cancel()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("interaction consumer did not stop")
		}
		client.Close()
	}
}

func hgInteractionRecord(t *testing.T, topic string, event InteractionEventsPackage.VideoInteractionChangedEvent) *kgo.Record {
	t.Helper()
	envelope, err := events.NewEnvelope(event)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	return &kgo.Record{Topic: topic, Key: []byte(event.EventKey()), Value: payload}
}

func hgAwaitInteractionState(t *testing.T, ctx context.Context, db *sql.DB, userID string, submissionID string, active bool, inboxRows int) {
	t.Helper()
	for ctx.Err() == nil {
		var actualActive bool
		var count int
		err := db.QueryRowContext(ctx, `SELECT active FROM video_user_interactions WHERE user_id = ? AND submission_id = ? AND interaction_type = 'like'`, userID, submissionID).Scan(&actualActive)
		if err == nil {
			_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM video_interaction_inbox WHERE event_key LIKE ?`, submissionID+":%").Scan(&count)
			if actualActive == active && count == inboxRows {
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("wait interaction state: %v", ctx.Err())
}

func hgCreateInteractionTopic(t *testing.T, ctx context.Context, brokers []string, topic string) {
	t.Helper()
	client, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	request := kmsg.NewPtrCreateTopicsRequest()
	request.Topics = append(request.Topics, kmsg.CreateTopicsRequestTopic{Topic: topic, NumPartitions: 1, ReplicationFactor: 3})
	response, err := request.RequestWith(ctx, client)
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Topics) != 1 {
		t.Fatalf("create topic response count = %d", len(response.Topics))
	}
	if topicErr := kerr.ErrorForCode(response.Topics[0].ErrorCode); topicErr != nil && topicErr != kerr.TopicAlreadyExists {
		t.Fatal(topicErr)
	}
	time.Sleep(500 * time.Millisecond)
}

func hgInteractionEnvOrDefault(key string, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
