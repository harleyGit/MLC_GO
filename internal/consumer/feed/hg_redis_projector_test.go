package feed

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
	err    error
}

func (s *hgRedisEvalStub) Eval(_ context.Context, script string, keys []string, args ...any) error {
	s.script = script
	s.keys = append([]string(nil), keys...)
	s.args = append([]any(nil), args...)
	return s.err
}

func TestRedisProjectorPublishesAtomicallyWithIdempotencyAndTrim(t *testing.T) {
	client := &hgRedisEvalStub{}
	projector := NewRedisProjector(client, 100000, 604800)

	if err := projector.Publish(context.Background(), "event-1", "submission-1", 1783152000123); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	wantKeys := []string{PersistenceRedisPackage.FeedPublishedZSetKey, PersistenceRedisPackage.GetFeedIdempotencyKey("event-1")}
	if client.script != PersistenceRedisPackage.FeedPublishLuaScript || !reflect.DeepEqual(client.keys, wantKeys) {
		t.Fatalf("script=%q keys=%v, want feed publish script and %v", client.script, client.keys, wantKeys)
	}
	wantArgs := []any{"submission-1", int64(1783152000123), 100000, int64(604800)}
	if !reflect.DeepEqual(client.args, wantArgs) {
		t.Fatalf("args=%#v, want %#v", client.args, wantArgs)
	}
}

func TestRedisProjectorDeletesAtomicallyWithIdempotency(t *testing.T) {
	client := &hgRedisEvalStub{}
	projector := NewRedisProjector(client, 100000, 604800)

	if err := projector.Delete(context.Background(), "event-2", "submission-2"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	wantKeys := []string{PersistenceRedisPackage.FeedPublishedZSetKey, PersistenceRedisPackage.GetFeedIdempotencyKey("event-2")}
	if client.script != PersistenceRedisPackage.FeedDeleteLuaScript || !reflect.DeepEqual(client.keys, wantKeys) {
		t.Fatalf("script=%q keys=%v, want feed delete script and %v", client.script, client.keys, wantKeys)
	}
}
