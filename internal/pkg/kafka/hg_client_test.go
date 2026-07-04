package HGKafkaPackage

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

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

func TestHGPingKafkaRequiresClient(t *testing.T) {
	oldClient := HGGlobalKgoClient
	HGGlobalKgoClient = nil
	t.Cleanup(func() { HGGlobalKgoClient = oldClient })

	err := HGPingKafka(context.Background())
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

func TestHGInitKafkaFailsWhenBrokerUnreachable(t *testing.T) {
	oldClient := HGGlobalKgoClient
	HGGlobalKgoClient = nil
	t.Cleanup(func() {
		HGCloseKafka()
		HGGlobalKgoClient = oldClient
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen unused port: %v", err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}

	start := time.Now()
	err = HGInitKafka(HGKafkaClusterConfig{Business: HGClusterConfig{Brokers: []string{addr}}})
	if err == nil {
		t.Fatal("expected unreachable broker to fail initialization")
	}
	if time.Since(start) > time.Second {
		t.Fatalf("expected fast broker reachability failure, took %s", time.Since(start))
	}
	if !strings.Contains(err.Error(), "ping") {
		t.Fatalf("expected ping error context, got %v", err)
	}
	if HGGlobalKgoClient != nil {
		t.Fatal("expected global kafka client to remain nil after failed init")
	}
}
