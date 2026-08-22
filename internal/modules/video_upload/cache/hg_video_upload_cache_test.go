package VideoUploadCachePackage

import (
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"context"
	"net"
	"reflect"
	"testing"

	"github.com/redis/go-redis/v9"
)

func TestVideoUploadCacheKeys(t *testing.T) {
	userID := "user_1"
	submissionID := "submission_1"

	if got := sessionKey(userID, submissionID); got != "video_upload:session:user_1:submission_1" {
		t.Fatalf("sessionKey() = %s", got)
	}
	if got := userRateKey(userID); got != "video_upload:rate:user:user_1" {
		t.Fatalf("userRateKey() = %s", got)
	}
	if got := ipRateKey("127.0.0.1"); got != "video_upload:rate:ip:127.0.0.1" {
		t.Fatalf("ipRateKey() = %s", got)
	}
	if got := submitLockKey(userID, submissionID); got != "video_upload:submit_lock:user_1:submission_1" {
		t.Fatalf("submitLockKey() = %s", got)
	}
	if got := submitResultKey(userID, submissionID); got != "video_upload:submit_result:user_1:submission_1" {
		t.Fatalf("submitResultKey() = %s", got)
	}
	if got := videoStatusCounterKey(); got != "video_status_counter" {
		t.Fatalf("videoStatusCounterKey() = %s", got)
	}
	if got := videoListPageKey("", 20, ""); got != "video_upload:list:cursor:first:size:20:tag:" {
		t.Fatalf("videoListPageKey(first) = %s", got)
	}
	if got := videoListPageKey("2026-07-04T10:00:00Z|submission_1", 20, "MMD·3D"); got != "video_upload:list:cursor:2026-07-04T10:00:00Z|submission_1:size:20:tag:MMD·3D" {
		t.Fatalf("videoListPageKey(cursor) = %s", got)
	}
}

type hgRedisCommandHook struct {
	command []interface{}
}

func (h *hgRedisCommandHook) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network string, addr string) (net.Conn, error) {
		return next(ctx, network, addr)
	}
}

func (h *hgRedisCommandHook) ProcessHook(redis.ProcessHook) redis.ProcessHook {
	return func(_ context.Context, cmd redis.Cmder) error {
		h.command = append([]interface{}(nil), cmd.Args()...)
		return nil
	}
}

func (h *hgRedisCommandHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}

func TestIncrementExternalCounterIfPresentUsesGuardedLua(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "unused"})
	defer client.Close()
	hook := &hgRedisCommandHook{}
	client.AddHook(hook)
	cache := &Cache{client: client}

	if err := cache.IncrementExternalCounterIfPresent(context.Background(), 3); err != nil {
		t.Fatalf("IncrementExternalCounterIfPresent() error = %v", err)
	}
	want := []interface{}{"eval", PersistenceRedisPackage.VideoExternalCounterIncrementIfPresentLuaScript, 1, "video_status_counter", int64(3)}
	if !reflect.DeepEqual(hook.command, want) {
		t.Fatalf("redis command = %#v, want %#v", hook.command, want)
	}

	hook.command = nil
	if err := cache.IncrementExternalCounterIfPresent(context.Background(), 0); err != nil {
		t.Fatalf("zero delta error = %v", err)
	}
	if hook.command != nil {
		t.Fatalf("zero delta redis command = %#v", hook.command)
	}
}
