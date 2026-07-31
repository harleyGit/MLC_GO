package CoinTaskPackage

import (
	CoinModelPackage "MLC_GO/internal/modules/coin/model"
	"context"
	"testing"
	"time"
)

type hgFakeJobRepository struct {
	users       []CoinModelPackage.HGUserCursor
	checkpoint  uint64
	expired     int
	drifts      []CoinModelPackage.HGWalletDrift
	initialized []string
}

type hgFakeJobLease struct{ acquired bool }

func (l *hgFakeJobLease) Acquire(context.Context, time.Duration) (string, bool, error) {
	return "token", l.acquired, nil
}
func (l *hgFakeJobLease) Release(context.Context, string) error { return nil }

func (f *hgFakeJobRepository) LoadInitializerCheckpoint(context.Context) (uint64, error) {
	return f.checkpoint, nil
}
func (f *hgFakeJobRepository) ListUsersAfter(context.Context, uint64, int) ([]CoinModelPackage.HGUserCursor, error) {
	return f.users, nil
}
func (f *hgFakeJobRepository) EnsureWallet(_ context.Context, userID string) error {
	f.initialized = append(f.initialized, userID)
	return nil
}
func (f *hgFakeJobRepository) SaveInitializerCheckpoint(_ context.Context, checkpoint uint64) error {
	f.checkpoint = checkpoint
	return nil
}
func (f *hgFakeJobRepository) ExpireBatch(context.Context, int, time.Time) (int, error) {
	return f.expired, nil
}
func (f *hgFakeJobRepository) ReconcileBatch(context.Context, uint64, int) ([]CoinModelPackage.HGWalletDrift, uint64, error) {
	return f.drifts, 0, nil
}

func TestHGJobsInitializerUsesBoundedCursorAndPersistsCheckpoint(t *testing.T) {
	repository := &hgFakeJobRepository{users: []CoinModelPackage.HGUserCursor{{ID: 41, UserID: "u-41"}, {ID: 42, UserID: "u-42"}}}
	jobs, err := NewHGJobs(repository, HGJobConfig{Interval: time.Minute, Timeout: time.Second, BatchSize: 2})
	if err != nil {
		t.Fatal(err)
	}

	if err := jobs.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if repository.checkpoint != 42 {
		t.Fatalf("checkpoint = %d, want 42", repository.checkpoint)
	}
	if len(repository.initialized) != 2 || repository.initialized[0] != "u-41" || repository.initialized[1] != "u-42" {
		t.Fatalf("initialized wallets = %#v", repository.initialized)
	}
}

func TestHGJobsReconciliationReportsDriftWithoutCorrection(t *testing.T) {
	repository := &hgFakeJobRepository{drifts: []CoinModelPackage.HGWalletDrift{{UserID: "u-1", WalletBalance: 9, LotBalance: 8}}}
	jobs, err := NewHGJobs(repository, HGJobConfig{Interval: time.Minute, Timeout: time.Second, BatchSize: 10})
	if err != nil {
		t.Fatal(err)
	}

	if err := jobs.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if repository.drifts[0].WalletBalance != 9 {
		t.Fatal("reconciliation must not silently mutate the wallet")
	}
}

func TestHGJobsSkipsDatabaseWorkWithoutDistributedLease(t *testing.T) {
	repository := &hgFakeJobRepository{users: []CoinModelPackage.HGUserCursor{{ID: 1, UserID: "u-1"}}}
	jobs, err := NewHGJobs(repository, HGJobConfig{Interval: time.Minute, Timeout: time.Second, BatchSize: 10}, &hgFakeJobLease{})
	if err != nil {
		t.Fatal(err)
	}
	if err := jobs.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(repository.initialized) != 0 {
		t.Fatalf("initialized = %#v, want no work without lease", repository.initialized)
	}
}
