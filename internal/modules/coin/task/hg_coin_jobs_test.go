package CoinTaskPackage

import (
	CoinModelPackage "MLC_GO/internal/modules/coin/model"
	"context"
	"testing"
	"time"
)

type hgFakeJobRepository struct {
	users               []CoinModelPackage.HGUserCursor
	checkpoint          uint64
	expired             int
	drifts              []CoinModelPackage.HGWalletDrift
	initialized         []string
	reconcileCheckpoint uint64
	consolidated        int
}

type hgFakeJobLease struct {
	acquired bool
	tasks    []string
}

func (l *hgFakeJobLease) Acquire(_ context.Context, task string, _ time.Duration) (string, bool, error) {
	l.tasks = append(l.tasks, task)
	return "token", l.acquired, nil
}
func (l *hgFakeJobLease) Release(context.Context, string, string) error { return nil }

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
	return f.drifts, 44, nil
}
func (f *hgFakeJobRepository) LoadReconciliationCheckpoint(context.Context) (uint64, error) {
	return f.reconcileCheckpoint, nil
}
func (f *hgFakeJobRepository) SaveReconciliationCheckpoint(_ context.Context, checkpoint uint64) error {
	f.reconcileCheckpoint = checkpoint
	return nil
}
func (f *hgFakeJobRepository) LoadConsolidationCheckpoint(context.Context) (uint64, error) {
	return 0, nil
}
func (f *hgFakeJobRepository) SaveConsolidationCheckpoint(context.Context, uint64) error { return nil }
func (f *hgFakeJobRepository) ConsolidateBatch(context.Context, uint64, int, int, uint64) (int, uint64, error) {
	f.consolidated++
	return 1, 9, nil
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
	if repository.reconcileCheckpoint != 44 {
		t.Fatalf("reconciliation checkpoint = %d, want 44", repository.reconcileCheckpoint)
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

func TestHGJobsConsolidationIsDisabledByDefault(t *testing.T) {
	repository := &hgFakeJobRepository{}
	jobs, err := NewHGJobs(repository, HGJobConfig{Interval: time.Minute, Timeout: time.Second, BatchSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if err := jobs.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if repository.consolidated != 0 {
		t.Fatal("consolidation must remain disabled by default")
	}
}

func TestHGJobsUsesTaskSpecificLeases(t *testing.T) {
	repository := &hgFakeJobRepository{}
	lease := &hgFakeJobLease{acquired: true}
	jobs, err := NewHGJobs(repository, HGJobConfig{Interval: time.Minute, Timeout: time.Second, BatchSize: 10, ConsolidationBatchSize: 2, ConsolidationSourceLimit: 8, ConsolidationMaxLotAmount: 10}, lease)
	if err != nil {
		t.Fatal(err)
	}
	if err := jobs.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := []string{"wallet_initializer", "lot_expiration", "wallet_reconciliation", "lot_consolidation"}
	if len(lease.tasks) != len(want) {
		t.Fatalf("lease tasks = %#v", lease.tasks)
	}
	for i := range want {
		if lease.tasks[i] != want[i] {
			t.Fatalf("lease tasks = %#v", lease.tasks)
		}
	}
}

func TestHGJobsStartRunsImmediately(t *testing.T) {
	repository := &hgFakeJobRepository{}
	called := make(chan struct{}, 1)
	repository.users = []CoinModelPackage.HGUserCursor{{ID: 1, UserID: "u-1"}}
	jobs, err := NewHGJobs(&hgNotifyingJobRepository{hgFakeJobRepository: repository, called: called}, HGJobConfig{Interval: time.Hour, Timeout: time.Second, BatchSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	jobs.Start(context.Background())
	defer jobs.Close()
	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("expected immediate startup run")
	}
}

type hgNotifyingJobRepository struct {
	*hgFakeJobRepository
	called chan struct{}
}

func (r *hgNotifyingJobRepository) LoadInitializerCheckpoint(ctx context.Context) (uint64, error) {
	select {
	case r.called <- struct{}{}:
	default:
	}
	return r.hgFakeJobRepository.LoadInitializerCheckpoint(ctx)
}
