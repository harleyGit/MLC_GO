package main

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestInitKafkaIfConfiguredRejectsEmptyConfig(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

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
