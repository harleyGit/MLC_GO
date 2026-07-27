package kafka

import (
	ClickHousePackage "MLC_GO/internal/pkg/clickhouse"
	HGKafkaPackage "MLC_GO/internal/pkg/kafka"
	"context"
	"strings"
	"testing"
)

type hgRuntimeStatisticStoreStub struct{}

func (hgRuntimeStatisticStoreStub) StoreStatisticEvent(context.Context, ClickHousePackage.HGStatisticEvent) error {
	return nil
}

type hgRuntimeRedisStub struct{}

func (hgRuntimeRedisStub) Eval(context.Context, string, []string, ...any) error { return nil }

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
	}, RuntimeDependencies{})
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	defer runtime.Close()
	if len(runtime.workers) != 0 {
		t.Fatalf("workers = %d, want 0", len(runtime.workers))
	}
}

func TestNewRuntimeCreatesEnabledFeedConsumer(t *testing.T) {
	runtime, err := NewRuntime(context.Background(), HGKafkaPackage.HGClusterConfig{
		Brokers: []string{"127.0.0.1:9092"},
		Topics:  []string{"mlc.domain.events"},
		Consumers: HGKafkaPackage.HGConsumerGroupConfigs{
			Feed: HGKafkaPackage.HGConsumerConfig{Enabled: true, GroupID: "feed", ClientID: "feed"},
		},
	}, RuntimeDependencies{Redis: hgRuntimeRedisStub{}})
	if err != nil {
		t.Fatalf("NewRuntime() error = %v", err)
	}
	defer runtime.Close()
	if len(runtime.workers) != 1 || runtime.workers[0].name != "feed" {
		t.Fatalf("workers = %#v, want one feed worker", runtime.workers)
	}
}

func TestNewRuntimeRejectsEnabledFeedWithoutRedis(t *testing.T) {
	_, err := NewRuntime(context.Background(), HGKafkaPackage.HGClusterConfig{
		Brokers: []string{"127.0.0.1:9092"},
		Topics:  []string{"mlc.domain.events"},
		Consumers: HGKafkaPackage.HGConsumerGroupConfigs{
			Feed: HGKafkaPackage.HGConsumerConfig{Enabled: true, GroupID: "feed", ClientID: "feed"},
		},
	}, RuntimeDependencies{})
	if err == nil || !strings.Contains(err.Error(), "redis") {
		t.Fatalf("error = %v, want redis dependency error", err)
	}
}

func TestNewRuntimeRejectsEnabledStatisticWithoutAuthorityStore(t *testing.T) {
	_, err := NewRuntime(context.Background(), HGKafkaPackage.HGClusterConfig{
		Brokers: []string{"127.0.0.1:9092"}, Topics: []string{"mlc.domain.events"},
		Consumers: HGKafkaPackage.HGConsumerGroupConfigs{Statistic: HGKafkaPackage.HGConsumerConfig{Enabled: true, GroupID: "statistic"}},
	}, RuntimeDependencies{Redis: hgRuntimeRedisStub{}})
	if err == nil || !strings.Contains(err.Error(), "ClickHouse") {
		t.Fatalf("error = %v, want ClickHouse dependency error", err)
	}
}

func TestNewRuntimeRejectsEnabledUnimplementedSearchConsumer(t *testing.T) {
	_, err := NewRuntime(context.Background(), HGKafkaPackage.HGClusterConfig{
		Brokers: []string{"127.0.0.1:9092"},
		Topics:  []string{"mlc.domain.events"},
		Consumers: HGKafkaPackage.HGConsumerGroupConfigs{
			Search: HGKafkaPackage.HGConsumerConfig{Enabled: true, GroupID: "search", ClientID: "search"},
		},
	}, RuntimeDependencies{Redis: hgRuntimeRedisStub{}})
	if err == nil || !strings.Contains(err.Error(), "not implemented") {
		t.Fatalf("error = %v, want not implemented", err)
	}
}
