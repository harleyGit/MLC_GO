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
	projector := NewRedisProjector(client, 64, 2000, "v2")
	delivery := Delivery{Topic: "mlc.domain.events", Partition: 7, Offset: 19}

	if err := projector.Publish(context.Background(), delivery, "event-1", "submission-1", 1783152000123); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	shard := PersistenceRedisPackage.GetFeedShard("submission-1", 64)
	wantKeys := []string{PersistenceRedisPackage.GetFeedPublishedZSetKey("v2", shard), PersistenceRedisPackage.GetFeedOffsetWatermarkKey("v2", shard)}
	if client.script != PersistenceRedisPackage.FeedPublishLuaScript || !reflect.DeepEqual(client.keys, wantKeys) {
		t.Fatalf("script=%q keys=%v, want feed publish script and %v", client.script, client.keys, wantKeys)
	}
	wantArgs := []any{"submission-1", int64(1783152000123), 2000, "mlc.domain.events:7", int64(19)}
	if !reflect.DeepEqual(client.args, wantArgs) {
		t.Fatalf("args=%#v, want %#v", client.args, wantArgs)
	}
}

func TestRedisProjectorDeletesAtomicallyWithIdempotency(t *testing.T) {
	client := &hgRedisEvalStub{}
	projector := NewRedisProjector(client, 64, 2000, "v2")
	delivery := Delivery{Topic: "mlc.domain.events", Partition: 8, Offset: 20}

	if err := projector.Delete(context.Background(), delivery, "event-2", "submission-2"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	shard := PersistenceRedisPackage.GetFeedShard("submission-2", 64)
	wantKeys := []string{PersistenceRedisPackage.GetFeedPublishedZSetKey("v2", shard), PersistenceRedisPackage.GetFeedOffsetWatermarkKey("v2", shard)}
	if client.script != PersistenceRedisPackage.FeedDeleteLuaScript || !reflect.DeepEqual(client.keys, wantKeys) {
		t.Fatalf("script=%q keys=%v, want feed delete script and %v", client.script, client.keys, wantKeys)
	}
	wantArgs := []any{"submission-2", "mlc.domain.events:8", int64(20)}
	if !reflect.DeepEqual(client.args, wantArgs) {
		t.Fatalf("args=%#v, want %#v", client.args, wantArgs)
	}
}

func TestFeedShardsDistributeSubmissionIDs(t *testing.T) {
	seen := make(map[int]struct{})
	for _, submissionID := range []string{"submission-1", "submission-2", "submission-3", "submission-4", "submission-5"} {
		seen[PersistenceRedisPackage.GetFeedShard(submissionID, 64)] = struct{}{}
	}
	if len(seen) < 2 {
		t.Fatalf("shards = %v, want multiple shards", seen)
	}
}
