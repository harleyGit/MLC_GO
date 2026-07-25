package kafka

import (
	HGKafkaPackage "MLC_GO/internal/pkg/kafka"
	"context"
	"strings"
	"testing"
)

func TestNewRuntimeDoesNotCreateDisabledConsumers(t *testing.T) {
	runtime, err := NewRuntime(context.Background(), HGKafkaPackage.HGClusterConfig{
		Brokers: []string{"127.0.0.1:9092"},
		Topics:  []string{"mlc.domain.events"},
		Consumers: HGKafkaPackage.HGConsumerGroupConfigs{
			Feed:      HGKafkaPackage.HGConsumerConfig{GroupID: "feed"},
			Search:    HGKafkaPackage.HGConsumerConfig{GroupID: "search"},
			Statistic: HGKafkaPackage.HGConsumerConfig{GroupID: "statistic"},
			Audit:     HGKafkaPackage.HGConsumerConfig{GroupID: "audit"},
		},
	})
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	defer runtime.Close()
	if len(runtime.workers) != 0 {
		t.Fatalf("workers = %d, want 0", len(runtime.workers))
	}
}

func TestNewRuntimeRejectsEnabledUnimplementedConsumer(t *testing.T) {
	_, err := NewRuntime(context.Background(), HGKafkaPackage.HGClusterConfig{
		Brokers: []string{"127.0.0.1:9092"},
		Topics:  []string{"mlc.domain.events"},
		Consumers: HGKafkaPackage.HGConsumerGroupConfigs{
			Feed: HGKafkaPackage.HGConsumerConfig{Enabled: true, GroupID: "feed"},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "not implemented") {
		t.Fatalf("error = %v, want not implemented", err)
	}
}
