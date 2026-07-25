package HGKafkaPackage

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
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

func hgEnvOrDefault(key string, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
