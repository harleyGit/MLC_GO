package HGKafkaPackage

import (
	UtilsPackage "MLC_GO/internal/pkg/utils"
	"context"
	"testing"

	"github.com/twmb/franz-go/pkg/kgo"
)

func TestHGInjectAndExtractTraceRecord(t *testing.T) {
	ctx := UtilsPackage.InjectTID(context.Background(), "tid-001")
	record := &kgo.Record{}

	HGInjectTraceToRecord(ctx, record)
	traceCtx := HGExtractTraceFromRecord(record)

	if got := UtilsPackage.GetTID(traceCtx); got != "tid-001" {
		t.Fatalf("expected tid-001, got %q", got)
	}
}

func TestHGInjectTraceSkipsEmptyTID(t *testing.T) {
	record := &kgo.Record{}

	HGInjectTraceToRecord(context.Background(), record)

	if len(record.Headers) != 0 {
		t.Fatalf("expected no headers, got %d", len(record.Headers))
	}
}
