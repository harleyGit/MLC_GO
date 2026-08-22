package runtime

import (
	CrawlerModelPackage "MLC_GO/internal/modules/crawler/model"
	"context"
	"testing"
	"time"
)

type hgSchedulerRepositoryStub struct {
	items []CrawlerModelPackage.HGTaskDefinition
}

func (s *hgSchedulerRepositoryStub) ListEnabledTaskDefinitions(context.Context, int) ([]CrawlerModelPackage.HGTaskDefinition, error) {
	return append([]CrawlerModelPackage.HGTaskDefinition(nil), s.items...), nil
}

type hgSchedulerRunnerStub struct{}

func (hgSchedulerRunnerStub) RunByID(context.Context, uint64) (*CrawlerModelPackage.HGTaskRun, error) {
	return nil, nil
}

func TestHGCrawlerTaskDigestIsOrderIndependentAndVersionSensitive(t *testing.T) {
	one := CrawlerModelPackage.HGTaskDefinition{ID: 1, Cron: "0 * * * * *", Version: 1}
	two := CrawlerModelPackage.HGTaskDefinition{ID: 2, Cron: "0 */5 * * * *", Version: 3}
	if hgCrawlerTaskDigest([]CrawlerModelPackage.HGTaskDefinition{one, two}) != hgCrawlerTaskDigest([]CrawlerModelPackage.HGTaskDefinition{two, one}) {
		t.Fatal("scheduler digest changed with repository order")
	}
	one.Version++
	if hgCrawlerTaskDigest([]CrawlerModelPackage.HGTaskDefinition{one, two}) == hgCrawlerTaskDigest([]CrawlerModelPackage.HGTaskDefinition{{ID: 1, Cron: one.Cron, Version: 1}, two}) {
		t.Fatal("scheduler digest ignored definition version")
	}
}

func TestHGTaskSchedulerRebuildsUTCEntries(t *testing.T) {
	repository := &hgSchedulerRepositoryStub{items: []CrawlerModelPackage.HGTaskDefinition{{ID: 7, Cron: "0 0 * * * *", Version: 1}}}
	scheduler, err := NewHGTaskScheduler(repository, hgSchedulerRunnerStub{}, true, time.Minute, 10)
	if err != nil {
		t.Fatal(err)
	}
	if err := scheduler.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = scheduler.Close() })
	scheduler.mu.Lock()
	defer scheduler.mu.Unlock()
	if scheduler.cron == nil || scheduler.cron.Location() != time.UTC || len(scheduler.cron.Entries()) != 1 {
		t.Fatalf("scheduler cron = %#v", scheduler.cron)
	}
}
