package VideoInteractionRepositoryPackage

import (
	"fmt"
	"io"
	"sync/atomic"
	"time"
)

var (
	hgPersistenceBatches  atomic.Uint64
	hgPersistenceEvents   atomic.Uint64
	hgPersistenceFailures atomic.Uint64
	hgPersistenceNanos    atomic.Uint64
	hgInboxDuplicates     atomic.Uint64
	hgInboxConflicts      atomic.Uint64
	hgWalletDebits        atomic.Uint64
)

func hgObservePersistenceBatch(events int, elapsed time.Duration, err error) {
	hgPersistenceBatches.Add(1)
	hgPersistenceEvents.Add(uint64(events))
	hgPersistenceNanos.Add(uint64(elapsed))
	if err != nil {
		hgPersistenceFailures.Add(1)
	}
}

func hgObserveInboxDuplicate() { hgInboxDuplicates.Add(1) }
func hgObserveInboxConflict()  { hgInboxConflicts.Add(1) }
func hgObserveWalletDebit()    { hgWalletDebits.Add(1) }

// HGWritePrometheusMetrics 输出 Interaction 持久化低基数进程指标。
func HGWritePrometheusMetrics(w io.Writer) {
	metrics := []struct {
		name  string
		help  string
		value uint64
	}{
		{"mlc_interaction_persistence_batches_total", "Interaction persistence transactions.", hgPersistenceBatches.Load()},
		{"mlc_interaction_persistence_events_total", "Interaction events included in persistence transactions.", hgPersistenceEvents.Load()},
		{"mlc_interaction_persistence_failures_total", "Failed interaction persistence transactions.", hgPersistenceFailures.Load()},
		{"mlc_interaction_persistence_batch_duration_nanoseconds_total", "Total interaction persistence duration in nanoseconds.", hgPersistenceNanos.Load()},
		{"mlc_interaction_inbox_duplicates_total", "Exact interaction event replays.", hgInboxDuplicates.Load()},
		{"mlc_interaction_inbox_conflicts_total", "Conflicting interaction inbox uniqueness collisions.", hgInboxConflicts.Load()},
		{"mlc_interaction_wallet_debits_total", "Committed coin wallet debits.", hgWalletDebits.Load()},
	}
	for _, metric := range metrics {
		_, _ = fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s counter\n%s %d\n", metric.name, metric.help, metric.name, metric.name, metric.value)
	}
}
