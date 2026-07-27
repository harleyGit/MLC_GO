package statistic

import (
	"fmt"
	"io"
	"sync/atomic"
)

var (
	hgStatisticAuthorityWrites       atomic.Uint64
	hgStatisticAuthorityWriteFailure atomic.Uint64
	hgStatisticRedisFailures         atomic.Uint64
	hgStatisticReconcileRuns         atomic.Uint64
	hgStatisticReconcileFailures     atomic.Uint64
	hgStatisticReconcileMismatches   atomic.Uint64
	hgStatisticReconcileLastSuccess  atomic.Int64
	hgStatisticReconcileCurrentDrift atomic.Uint64
)

// HGStatisticMetrics 是 Statistic 权威写入和对账的进程级指标快照。
type HGStatisticMetrics struct {
	AuthorityWrites       uint64
	AuthorityWriteFailure uint64
	RedisFailures         uint64
	ReconcileRuns         uint64
	ReconcileFailures     uint64
	ReconcileMismatches   uint64
	ReconcileLastSuccess  int64
	CurrentDrift          uint64
}

// HGStatisticMetricsSnapshot 返回无锁原子指标快照。
func HGStatisticMetricsSnapshot() HGStatisticMetrics {
	return HGStatisticMetrics{
		AuthorityWrites: hgStatisticAuthorityWrites.Load(), AuthorityWriteFailure: hgStatisticAuthorityWriteFailure.Load(),
		RedisFailures: hgStatisticRedisFailures.Load(), ReconcileRuns: hgStatisticReconcileRuns.Load(),
		ReconcileFailures: hgStatisticReconcileFailures.Load(), ReconcileMismatches: hgStatisticReconcileMismatches.Load(),
		ReconcileLastSuccess: hgStatisticReconcileLastSuccess.Load(), CurrentDrift: hgStatisticReconcileCurrentDrift.Load(),
	}
}

// HGWritePrometheusMetrics 输出固定低基数的 Statistic 指标。
func HGWritePrometheusMetrics(w io.Writer) {
	metrics := HGStatisticMetricsSnapshot()
	writeCounter := func(name string, help string, value uint64) {
		_, _ = fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s counter\n%s %d\n", name, help, name, name, value)
	}
	writeGauge := func(name string, help string, value int64) {
		_, _ = fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s gauge\n%s %d\n", name, help, name, name, value)
	}
	writeCounter("mlc_statistic_clickhouse_writes_total", "Statistic authority events acknowledged by ClickHouse.", metrics.AuthorityWrites)
	writeCounter("mlc_statistic_clickhouse_write_failures_total", "Statistic authority writes rejected or unavailable.", metrics.AuthorityWriteFailure)
	writeCounter("mlc_statistic_redis_projection_failures_total", "Statistic Redis projection failures after authority writes.", metrics.RedisFailures)
	writeCounter("mlc_statistic_reconcile_runs_total", "Statistic Redis and ClickHouse reconciliation runs.", metrics.ReconcileRuns)
	writeCounter("mlc_statistic_reconcile_failures_total", "Statistic reconciliation failures.", metrics.ReconcileFailures)
	writeCounter("mlc_statistic_reconcile_mismatches_total", "Statistic dimensions observed with drift.", metrics.ReconcileMismatches)
	writeGauge("mlc_statistic_reconcile_last_success_unixtime", "Unix time of the last successful statistic reconciliation.", metrics.ReconcileLastSuccess)
	writeGauge("mlc_statistic_reconcile_current_drift", "Current absolute Redis versus ClickHouse statistic drift.", int64(metrics.CurrentDrift))
}

func hgResetStatisticMetricsForTest() {
	hgStatisticAuthorityWrites.Store(0)
	hgStatisticAuthorityWriteFailure.Store(0)
	hgStatisticRedisFailures.Store(0)
	hgStatisticReconcileRuns.Store(0)
	hgStatisticReconcileFailures.Store(0)
	hgStatisticReconcileMismatches.Store(0)
	hgStatisticReconcileLastSuccess.Store(0)
	hgStatisticReconcileCurrentDrift.Store(0)
}
