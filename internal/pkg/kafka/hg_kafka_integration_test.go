package HGKafkaPackage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
)

func TestHGKafkaIntegrationProduceConsumeCommit(t *testing.T) {
	if os.Getenv("MLC_KAFKA_INTEGRATION") != "1" {
		t.Skip("set MLC_KAFKA_INTEGRATION=1 to run Kafka integration tests")
	}
	brokers := strings.Split(hgEnvOrDefault("MLC_KAFKA_BROKERS", "localhost:19092,localhost:29092,localhost:39092"), ",")
	topic := fmt.Sprintf("mlc.integration.%d", time.Now().UnixNano())
	groupID := fmt.Sprintf("mlc-integration-%d", time.Now().UnixNano())

	producer, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	if err != nil {
		t.Fatalf("new producer: %v", err)
	}
	defer producer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := producer.ProduceSync(ctx, &kgo.Record{Topic: topic, Key: []byte("key"), Value: []byte("value")}).FirstErr(); err != nil {
		t.Fatalf("produce: %v", err)
	}

	opts, err := HGNewBusinessConsumerOpts(HGClusterConfig{Brokers: brokers}, []string{topic}, groupID, groupID)
	if err != nil {
		t.Fatalf("consumer opts: %v", err)
	}
	consumer, err := kgo.NewClient(opts...)
	if err != nil {
		t.Fatalf("new consumer: %v", err)
	}
	defer consumer.Close()

	for {
		fetches := consumer.PollRecords(ctx, 10)
		if err := fetches.Err(); err != nil {
			t.Fatalf("poll: %v", err)
		}
		records := fetches.Records()
		if len(records) == 0 {
			continue
		}
		if string(records[0].Value) != "value" {
			t.Fatalf("value = %q, want value", records[0].Value)
		}
		if err := consumer.CommitRecords(ctx, records...); err != nil {
			t.Fatalf("commit: %v", err)
		}
		consumer.AllowRebalance()
		break
	}
}

func TestHGKafkaIntegrationCommittedOffsetSurvivesConsumerRestart(t *testing.T) {
	brokers := hgKafkaIntegrationBrokers(t)
	topic := fmt.Sprintf("mlc.integration.offset.%d", time.Now().UnixNano())
	groupID := fmt.Sprintf("mlc-integration-offset-%d", time.Now().UnixNano())
	hgCreateIntegrationTopic(t, brokers, topic, 2)

	producer := hgNewIntegrationClient(t, kgo.SeedBrokers(brokers...))
	defer producer.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if err := producer.ProduceSync(ctx,
		&kgo.Record{Topic: topic, Partition: 0, Key: []byte("p0-old"), Value: []byte("old-0")},
		&kgo.Record{Topic: topic, Partition: 1, Key: []byte("p1-old"), Value: []byte("old-1")},
	).FirstErr(); err != nil {
		t.Fatalf("produce old records: %v", err)
	}

	consumerA := hgNewIntegrationGroupConsumer(t, brokers, topic, groupID)
	hgConsumeAndCommitValues(t, ctx, consumerA, map[string]bool{"old-0": true, "old-1": true})
	consumerA.Close()

	if err := producer.ProduceSync(ctx,
		&kgo.Record{Topic: topic, Partition: 0, Key: []byte("p0-new"), Value: []byte("new-0")},
		&kgo.Record{Topic: topic, Partition: 1, Key: []byte("p1-new"), Value: []byte("new-1")},
	).FirstErr(); err != nil {
		t.Fatalf("produce new records: %v", err)
	}

	consumerB := hgNewIntegrationGroupConsumer(t, brokers, topic, groupID)
	defer consumerB.Close()
	hgConsumeAndCommitValues(t, ctx, consumerB, map[string]bool{"new-0": true, "new-1": true})
	committed := consumerB.CommittedOffsets()[topic]
	for partition := int32(0); partition < 2; partition++ {
		if committed[partition].Offset < 2 {
			t.Fatalf("partition %d committed offset=%d, want at least 2", partition, committed[partition].Offset)
		}
	}
}

func TestHGKafkaIntegrationTwoConsumersRebalance(t *testing.T) {
	brokers := hgKafkaIntegrationBrokers(t)
	topic := fmt.Sprintf("mlc.integration.rebalance.%d", time.Now().UnixNano())
	groupID := fmt.Sprintf("mlc-integration-rebalance-%d", time.Now().UnixNano())
	hgCreateIntegrationTopic(t, brokers, topic, 2)

	assignedA := make(chan int, 4)
	assignedB := make(chan int, 4)
	consumerA := hgNewIntegrationRebalanceConsumer(t, brokers, topic, groupID, assignedA)
	defer consumerA.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	go hgPollIntegrationConsumer(ctx, consumerA)
	hgWaitAssignedPartitions(t, ctx, assignedA, 2)

	consumerB := hgNewIntegrationRebalanceConsumer(t, brokers, topic, groupID, assignedB)
	defer consumerB.Close()
	go hgPollIntegrationConsumer(ctx, consumerB)
	hgWaitAssignedPartitions(t, ctx, assignedB, 1)

	// 两分区、两成员时每个成员至少应获得一个分区，证明真实 group rebalance 已发生。
	select {
	case count := <-assignedA:
		if count < 1 {
			t.Fatalf("consumer A assigned %d partitions after rebalance, want at least 1", count)
		}
	case <-ctx.Done():
		t.Fatalf("wait consumer A reassignment: %v", ctx.Err())
	}
}

func hgKafkaIntegrationBrokers(t *testing.T) []string {
	t.Helper()
	if os.Getenv("MLC_KAFKA_INTEGRATION") != "1" {
		t.Skip("set MLC_KAFKA_INTEGRATION=1 to run Kafka integration tests")
	}
	return strings.Split(hgEnvOrDefault("MLC_KAFKA_BROKERS", "localhost:19092,localhost:29092,localhost:39092"), ",")
}

func hgCreateIntegrationTopic(t *testing.T, brokers []string, topic string, partitions int32) {
	t.Helper()
	client := hgNewIntegrationClient(t, kgo.SeedBrokers(brokers...))
	defer client.Close()
	request := kmsg.NewPtrCreateTopicsRequest()
	request.Topics = append(request.Topics, kmsg.CreateTopicsRequestTopic{Topic: topic, NumPartitions: partitions, ReplicationFactor: 3})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	response, err := request.RequestWith(ctx, client)
	if err != nil {
		t.Fatalf("create topic %q: %v", topic, err)
	}
	if len(response.Topics) != 1 {
		t.Fatalf("create topic response count=%d, want 1", len(response.Topics))
	}
	if topicErr := kerr.ErrorForCode(response.Topics[0].ErrorCode); topicErr != nil && !errors.Is(topicErr, kerr.TopicAlreadyExists) {
		t.Fatalf("create topic %q: %v", topic, topicErr)
	}
}

func hgNewIntegrationClient(t *testing.T, opts ...kgo.Opt) *kgo.Client {
	t.Helper()
	client, err := kgo.NewClient(opts...)
	if err != nil {
		t.Fatalf("new kafka client: %v", err)
	}
	return client
}

func hgNewIntegrationGroupConsumer(t *testing.T, brokers []string, topic string, groupID string) *kgo.Client {
	t.Helper()
	opts, err := HGNewBusinessConsumerOpts(HGClusterConfig{Brokers: brokers}, []string{topic}, groupID, groupID)
	if err != nil {
		t.Fatalf("consumer opts: %v", err)
	}
	return hgNewIntegrationClient(t, opts...)
}

func hgConsumeAndCommitValues(t *testing.T, ctx context.Context, client *kgo.Client, expected map[string]bool) {
	t.Helper()
	seen := make(map[string]bool, len(expected))
	for len(seen) < len(expected) {
		fetches := client.PollRecords(ctx, len(expected))
		if err := fetches.Err(); err != nil {
			t.Fatalf("poll records: %v", err)
		}
		records := fetches.Records()
		for _, record := range records {
			value := string(record.Value)
			if !expected[value] {
				t.Fatalf("unexpected replayed value %q", value)
			}
			seen[value] = true
		}
		if len(records) > 0 {
			if err := client.CommitRecords(ctx, records...); err != nil {
				t.Fatalf("commit records: %v", err)
			}
		}
		client.AllowRebalance()
	}
}

func hgNewIntegrationRebalanceConsumer(t *testing.T, brokers []string, topic string, groupID string, assigned chan<- int) *kgo.Client {
	t.Helper()
	return hgNewIntegrationClient(t,
		kgo.SeedBrokers(brokers...),
		kgo.ConsumeTopics(topic),
		kgo.ConsumerGroup(groupID),
		kgo.DisableAutoCommit(),
		kgo.OnPartitionsAssigned(func(_ context.Context, _ *kgo.Client, partitions map[string][]int32) {
			count := 0
			for _, topicPartitions := range partitions {
				count += len(topicPartitions)
			}
			assigned <- count
		}),
	)
}

func hgPollIntegrationConsumer(ctx context.Context, client *kgo.Client) {
	for ctx.Err() == nil {
		client.PollRecords(ctx, 10)
	}
}

func hgWaitAssignedPartitions(t *testing.T, ctx context.Context, assigned <-chan int, minimum int) {
	t.Helper()
	for {
		select {
		case count := <-assigned:
			if count >= minimum {
				return
			}
		case <-ctx.Done():
			t.Fatalf("wait assigned partitions: %v", ctx.Err())
		}
	}
}

func hgEnvOrDefault(key string, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
