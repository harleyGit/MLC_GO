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
	counter := NewRedisCounter(client, 604800)

	if err := counter.Increment(context.Background(), "event-1", "video.published"); err != nil {
		t.Fatalf("Increment() error = %v", err)
	}
	wantKeys := []string{PersistenceRedisPackage.VideoEventCounterKey, PersistenceRedisPackage.GetVideoStatisticIdempotencyKey("event-1")}
	if client.script != PersistenceRedisPackage.VideoStatisticIncrementLuaScript || !reflect.DeepEqual(client.keys, wantKeys) {
		t.Fatalf("script=%q keys=%v, want statistic script and %v", client.script, client.keys, wantKeys)
	}
	wantArgs := []any{"video.published", int64(604800)}
	if !reflect.DeepEqual(client.args, wantArgs) {
		t.Fatalf("args=%#v, want %#v", client.args, wantArgs)
	}
}
