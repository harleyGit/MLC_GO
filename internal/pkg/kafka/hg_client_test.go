package HGKafkaPackage

import (
	"context"
	"testing"

	"github.com/twmb/franz-go/pkg/kgo"
)

func TestHGSendBusinessEventRequiresClient(t *testing.T) {
	oldClient := HGGlobalKgoClient
	HGGlobalKgoClient = nil
	t.Cleanup(func() { HGGlobalKgoClient = oldClient })

	err := HGSendBusinessEvent(context.Background(), "topic", "key", map[string]string{"ok": "1"})
	if err == nil {
		t.Fatal("expected error when kafka client is nil")
	}
}

func TestHGBuildRecordMarshalsPayloadAndTrace(t *testing.T) {
	record, err := HGBuildRecord(context.Background(), "topic-a", "key-a", map[string]string{"hello": "world"})
	if err != nil {
		t.Fatalf("expected record, got %v", err)
	}

	if record.Topic != "topic-a" || string(record.Key) != "key-a" {
		t.Fatalf("unexpected record identity: topic=%s key=%s", record.Topic, string(record.Key))
	}

	if string(record.Value) != `{"hello":"world"}` {
		t.Fatalf("unexpected value %s", string(record.Value))
	}
}

func TestHGCloseKafkaFlushesAndClosesClient(t *testing.T) {
	oldClient := HGGlobalKgoClient
	client, err := kgo.NewClient(kgo.SeedBrokers("127.0.0.1:9092"))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	HGGlobalKgoClient = client
	t.Cleanup(func() { HGGlobalKgoClient = oldClient })

	HGCloseKafka()

	if HGGlobalKgoClient != nil {
		t.Fatal("expected global client reset to nil")
	}
}
