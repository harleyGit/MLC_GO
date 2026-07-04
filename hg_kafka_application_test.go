package main

import "testing"

func TestInitKafkaIfConfiguredSkipsEmptyConfig(t *testing.T) {
	closer, err := initKafkaIfConfigured()
	if err != nil {
		t.Fatalf("expected empty kafka config to be skipped, got %v", err)
	}

	if closer != nil {
		t.Fatal("expected nil closer when kafka is not configured")
	}
}
