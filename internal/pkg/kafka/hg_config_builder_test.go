package HGKafkaPackage

import (
	"reflect"
	"regexp"
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

func TestHGNewBusinessClientOptsConfiguresManualConsumerGroup(t *testing.T) {
	cfg := HGClusterConfig{
		Brokers: []string{"127.0.0.1:9092"},
		Topics:  []string{"mlc.domain.events"},
		GroupID: "mlc-go-domain-events",
	}
	opts, err := HGNewBusinessClientOpts(cfg)
	if err != nil {
		t.Fatalf("expected valid business client opts, got %v", err)
	}

	client, err := kgo.NewClient(opts...)
	if err != nil {
		t.Fatalf("expected franz-go client from opts, got %v", err)
	}
	defer client.Close()

	if got := client.OptValue(kgo.ConsumeTopics); !reflect.DeepEqual(got, map[string]*regexp.Regexp{"mlc.domain.events": nil}) {
		t.Fatalf("consume topics = %v, want mlc.domain.events", got)
	}
	if got := client.OptValue(kgo.ConsumerGroup); got != "mlc-go-domain-events" {
		t.Fatalf("consumer group = %v, want mlc-go-domain-events", got)
	}
	if got := client.OptValue(kgo.DisableAutoCommit); got != true {
		t.Fatalf("disable auto commit = %v, want true", got)
	}
	if got := client.OptValue(kgo.BlockRebalanceOnPoll); got != true {
		t.Fatalf("block rebalance on poll = %v, want true", got)
	}
}

func TestHGNewBusinessClientOptsRequiresConsumerSettings(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		cfg     HGClusterConfig
		wantErr string
	}{
		{
			name:    "topics",
			cfg:     HGClusterConfig{Brokers: []string{"127.0.0.1:9092"}, GroupID: "mlc-go-domain-events"},
			wantErr: "topics",
		},
		{
			name:    "group id",
			cfg:     HGClusterConfig{Brokers: []string{"127.0.0.1:9092"}, Topics: []string{"mlc.domain.events"}},
			wantErr: "group_id",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := HGNewBusinessClientOpts(testCase.cfg)
			if err == nil || !strings.Contains(err.Error(), testCase.wantErr) {
				t.Fatalf("error = %v, want containing %q", err, testCase.wantErr)
			}
		})
	}
}
