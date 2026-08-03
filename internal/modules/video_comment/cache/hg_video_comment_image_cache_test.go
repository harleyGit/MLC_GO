package VideoCommentCachePackage

import (
	"context"
	"testing"
	"time"
)

type hgFakeRateEval struct {
	keys    []string
	results []int64
}

func (f *hgFakeRateEval) EvalInt64(_ context.Context, _ string, keys []string, _ ...any) (int64, error) {
	f.keys = append(f.keys, keys[0])
	result := f.results[0]
	f.results = f.results[1:]
	return result, nil
}

func TestHGImageRateLimiterChecksUserAndSourceIPBuckets(t *testing.T) {
	eval := &hgFakeRateEval{results: []int64{1, 1}}
	limiter, err := NewHGImageRateLimiter(eval, HGImageRateLimitConfig{
		UserCapacity: 6, IPCapacity: 30, Window: time.Minute,
	})
	if err != nil {
		t.Fatalf("NewHGImageRateLimiter() error=%v", err)
	}
	if err := limiter.Allow(context.Background(), "user-1", "203.0.113.8"); err != nil {
		t.Fatalf("Allow() error=%v", err)
	}
	if len(eval.keys) != 2 || eval.keys[0] != "video_comment:image:rate:user:user-1" || eval.keys[1] != "video_comment:image:rate:ip:203.0.113.8" {
		t.Fatalf("Allow() keys=%v", eval.keys)
	}
}

func TestHGImageRateLimiterFailsClosedWhenBucketIsExhausted(t *testing.T) {
	eval := &hgFakeRateEval{results: []int64{0}}
	limiter, err := NewHGImageRateLimiter(eval, HGImageRateLimitConfig{UserCapacity: 6, IPCapacity: 30, Window: time.Minute})
	if err != nil {
		t.Fatalf("NewHGImageRateLimiter() error=%v", err)
	}
	if err := limiter.Allow(context.Background(), "user-1", "203.0.113.8"); err != ErrImageRateLimited {
		t.Fatalf("Allow() error=%v, want ErrImageRateLimited", err)
	}
}
