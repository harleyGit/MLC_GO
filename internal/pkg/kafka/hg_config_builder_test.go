package HGKafkaPackage

import (
	"strings"
	"testing"

	"github.com/twmb/franz-go/pkg/kgo"
)

func TestHGBuildClusterConfigRequiresBrokers(t *testing.T) {
	_, err := HGBuildClusterConfig(HGClusterConfig{})
	if err == nil {
		t.Fatal("expected error when brokers are empty")
	}

	if !strings.Contains(err.Error(), "brokers") {
		t.Fatalf("expected brokers error, got %v", err)
	}
}

func TestHGBuildClusterConfigRejectsInvalidAcks(t *testing.T) {
	_, err := HGBuildClusterConfig(HGClusterConfig{Brokers: []string{"127.0.0.1:9092"}, Acks: "many"})
	if err == nil {
		t.Fatal("expected error when acks is invalid")
	}

	if !strings.Contains(err.Error(), "acks") {
		t.Fatalf("expected acks error, got %v", err)
	}
}

func TestHGNewProducerOptsIncludesSeedBrokers(t *testing.T) {
	cfg := HGClusterConfig{Brokers: []string{"127.0.0.1:9092"}, Retry: 3}
	opts, err := HGNewBusinessProducerOpts(cfg)
	if err != nil {
		t.Fatalf("expected valid opts, got %v", err)
	}

	client, err := kgo.NewClient(opts...)
	if err != nil {
		t.Fatalf("expected franz-go client from opts, got %v", err)
	}
	client.Close()
}
