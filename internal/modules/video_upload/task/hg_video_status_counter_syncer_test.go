package VideoUploadTaskPackage

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func TestStatusCounterSyncerRequiresDependencies(t *testing.T) {
	syncer := NewStatusCounterSyncer(nil, nil, 0)
	if syncer != nil {
		t.Fatalf("NewStatusCounterSyncer() should return nil without dependencies")
	}
}

func TestStatusCounterSyncerSyncsRepoCountersToCache(t *testing.T) {
	repo := &fakeStatusCounterRepo{counters: map[string]int64{"reviewing": 3, "published": 2}}
	cache := &fakeStatusCounterCache{}
	syncer := NewStatusCounterSyncer(repo, cache, time.Minute)

	syncer.sync(context.Background())

	if !reflect.DeepEqual(cache.counters, repo.counters) {
		t.Fatalf("sync counters = %#v, want %#v", cache.counters, repo.counters)
	}
}

type fakeStatusCounterRepo struct {
	counters map[string]int64
}

func (r *fakeStatusCounterRepo) GetVideoStatusCounts(ctx context.Context) (map[string]int64, error) {
	return r.counters, nil
}

type fakeStatusCounterCache struct {
	counters map[string]int64
}

func (c *fakeStatusCounterCache) SetVideoStatusCounters(ctx context.Context, counters map[string]int64) error {
	c.counters = counters
	return nil
}
