package service

import (
	CrawlerDtoPackage "MLC_GO/internal/modules/crawler/dto"
	"context"
	"errors"
	"testing"
)

func TestHGDebugRejectsPrivateTargets(t *testing.T) {
	service := NewHGDebugService()
	for _, target := range []string{"http://127.0.0.1/test", "http://10.0.0.1/test", "http://[::1]/test"} {
		_, err := service.TestRequest(context.Background(), CrawlerDtoPackage.HGDebugRequest{URL: target, Method: "GET"})
		if !errors.Is(err, ErrHGDebugUnsafeTarget) {
			t.Fatalf("target %s error = %v, want unsafe target", target, err)
		}
	}
}

func TestHGDetectDebugFieldsUsesFirstArrayItem(t *testing.T) {
	fields := make([]CrawlerDtoPackage.HGDetectedField, 0)
	hgDetectDebugFields(map[string]any{"data": []any{map[string]any{"id": "BV1", "owner": map[string]any{"name": "author"}}}}, "$", 0, &fields)
	if len(fields) != 2 || fields[0].Path != "$.data[*].id" || fields[1].Path != "$.data[*].owner.name" {
		t.Fatalf("fields = %#v", fields)
	}
}
