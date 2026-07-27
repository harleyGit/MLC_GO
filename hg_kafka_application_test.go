package main

import (
	HGKafkaPackage "MLC_GO/internal/pkg/kafka"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestBuildManagementListenAddrUsesExplicitHost(t *testing.T) {
	if got := buildManagementListenAddr("127.0.0.1", "9091"); got != "127.0.0.1:9091" {
		t.Fatalf("buildManagementListenAddr() = %q, want 127.0.0.1:9091", got)
	}
}

func TestInitKafkaIfConfiguredRejectsEmptyConfig(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	t.Setenv("SERVER_ENV", "prod")

	closer, err := initKafkaIfConfigured()
	if err == nil {
		t.Fatal("expected empty kafka config to prevent initialization")
	}
	if closer != nil {
		t.Fatal("expected nil closer when required kafka config is missing")
	}
	if !strings.Contains(err.Error(), "business.brokers") {
		t.Fatalf("expected missing business.brokers error, got %v", err)
	}
}

func TestInitKafkaIfConfiguredAllowsDebugWithoutBroker(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	t.Setenv("SERVER_ENV", "debug")
	t.Setenv("KAFKA_REQUIRED", "")
	setValidKafkaConfig()

	closer, err := hgInitKafkaIfConfigured(func(HGKafkaPackage.HGKafkaClusterConfig) error {
		return errors.New("broker unavailable")
	})
	if err != nil {
		t.Fatalf("expected debug startup to degrade without kafka, got %v", err)
	}
	if closer != nil {
		t.Fatal("expected nil closer when kafka initialization is skipped")
	}
}

func TestInitKafkaIfConfiguredRejectsBrokerFailureWhenRequired(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		env      string
		required string
	}{
		{name: "debug override", env: "debug", required: "true"},
		{name: "pre", env: "pre"},
		{name: "prod", env: "prod"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			viper.Reset()
			t.Cleanup(viper.Reset)
			t.Setenv("SERVER_ENV", testCase.env)
			t.Setenv("KAFKA_REQUIRED", testCase.required)
			setValidKafkaConfig()

			closer, err := hgInitKafkaIfConfigured(func(HGKafkaPackage.HGKafkaClusterConfig) error {
				return errors.New("broker unavailable")
			})
			if err == nil {
				t.Fatal("expected required kafka failure to prevent startup")
			}
			if closer != nil {
				t.Fatal("expected nil closer after kafka initialization failure")
			}
			if !strings.Contains(err.Error(), "broker unavailable") {
				t.Fatalf("expected broker failure context, got %v", err)
			}
		})
	}
}

func setValidKafkaConfig() {
	viper.Set("kafka.business.brokers", []string{"127.0.0.1:19092"})
	viper.Set("kafka.business.acks", "all")
	viper.Set("kafka.business.client_id", "mlc-go-test-business")
	viper.Set("kafka.business.topics", []string{"mlc.domain.events"})
	viper.Set("kafka.business.consumers.feed.group_id", "mlc-go-test-feed")
	viper.Set("kafka.business.consumers.search.group_id", "mlc-go-test-search")
	viper.Set("kafka.business.consumers.statistic.group_id", "mlc-go-test-statistic")
	viper.Set("kafka.business.consumers.audit.group_id", "mlc-go-test-audit")
	viper.Set("kafka.log.brokers", []string{"127.0.0.1:19092"})
	viper.Set("kafka.log.acks", "1")
	viper.Set("kafka.log.client_id", "mlc-go-test-log")
}
