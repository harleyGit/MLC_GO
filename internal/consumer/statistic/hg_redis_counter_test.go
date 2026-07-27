package statistic

import (
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"context"
	"reflect"
	"testing"
)

type hgRedisEvalStub struct {
	script string
	keys   []string
	args   []any
}

func (s *hgRedisEvalStub) Eval(_ context.Context, script string, keys []string, args ...any) error {
	s.script = script
	s.keys = append([]string(nil), keys...)
	s.args = append([]any(nil), args...)
	return nil
}

func TestRedisCounterIncrementsAtomicallyWithEventIdempotency(t *testing.T) {
	client := &hgRedisEvalStub{}
	counter := NewRedisCounter(client, 64, "v2")
	delivery := Delivery{Topic: "mlc.domain.events", Partition: 3, Offset: 11}

	if err := counter.Increment(context.Background(), delivery, "event-1", "video.published"); err != nil {
		t.Fatalf("Increment() error = %v", err)
	}
	shard := PersistenceRedisPackage.GetStatisticShard(delivery.Partition, 64)
	wantKeys := []string{PersistenceRedisPackage.GetVideoEventCounterKey("v2", shard), PersistenceRedisPackage.GetVideoStatisticOffsetWatermarkKey("v2", shard)}
	if client.script != PersistenceRedisPackage.VideoStatisticIncrementLuaScript || !reflect.DeepEqual(client.keys, wantKeys) {
		t.Fatalf("script=%q keys=%v, want statistic script and %v", client.script, client.keys, wantKeys)
	}
	wantArgs := []any{"video.published", "mlc.domain.events:3", int64(11)}
	if !reflect.DeepEqual(client.args, wantArgs) {
		t.Fatalf("args=%#v, want %#v", client.args, wantArgs)
	}
}
