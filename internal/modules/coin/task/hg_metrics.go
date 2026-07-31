package CoinTaskPackage

import (
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

var hgReconciliationDrifts atomic.Uint64

type hgCoinJobMetric struct {
	success    atomic.Uint64
	failure    atomic.Uint64
	processed  atomic.Uint64
	leaseSkips atomic.Uint64
	nanos      atomic.Uint64
}

var (
	hgCoinJobMetricsMu sync.Mutex
	// Keys are restricted to the fixed job catalog emitted below; never add user, request, or lot identifiers.
	hgCoinJobMetrics = map[string]*hgCoinJobMetric{}
)

func hgObserveReconciliationDrift(count int) { hgReconciliationDrifts.Add(uint64(count)) }

func hgObserveCoinJobRun(job string, elapsed time.Duration, err error) {
	metric := hgCoinJobMetricFor(job)
	metric.nanos.Add(uint64(elapsed))
	if err != nil {
		metric.failure.Add(1)
		return
	}
	metric.success.Add(1)
}

func hgObserveCoinJobProcessed(job string, count int) {
	if count > 0 {
		hgCoinJobMetricFor(job).processed.Add(uint64(count))
	}
}

func hgObserveCoinJobLeaseSkip(job string) { hgCoinJobMetricFor(job).leaseSkips.Add(1) }

func hgCoinJobMetricFor(job string) *hgCoinJobMetric {
	hgCoinJobMetricsMu.Lock()
	defer hgCoinJobMetricsMu.Unlock()
	metric := hgCoinJobMetrics[job]
	if metric == nil {
		metric = &hgCoinJobMetric{}
		hgCoinJobMetrics[job] = metric
	}
	return metric
}

// HGWritePrometheusMetrics 输出只检测对账的累计漂移数，不包含用户标签或自动修复结果。
func HGWritePrometheusMetrics(w io.Writer) {
	_, _ = fmt.Fprintf(w, "mlc_coin_reconciliation_drifts_total %d\n", hgReconciliationDrifts.Load())
	jobs := [...]string{"wallet_initializer", "lot_expiration", "wallet_reconciliation", "lot_consolidation"}
	for _, job := range jobs {
		metric := hgCoinJobMetricFor(job)
		_, _ = fmt.Fprintf(w, "mlc_coin_job_runs_total{job=\"%s\",result=\"success\"} %d\n", job, metric.success.Load())
		_, _ = fmt.Fprintf(w, "mlc_coin_job_runs_total{job=\"%s\",result=\"failure\"} %d\n", job, metric.failure.Load())
		_, _ = fmt.Fprintf(w, "mlc_coin_job_processed_total{job=\"%s\"} %d\n", job, metric.processed.Load())
		_, _ = fmt.Fprintf(w, "mlc_coin_job_lease_skips_total{job=\"%s\"} %d\n", job, metric.leaseSkips.Load())
		_, _ = fmt.Fprintf(w, "mlc_coin_job_duration_nanoseconds_total{job=\"%s\"} %d\n", job, metric.nanos.Load())
	}
}

// HGReconciliationMetricsSnapshot 返回无用户维度的累计漂移快照，供受鉴权运维 API 展示。
func HGReconciliationMetricsSnapshot() uint64 { return hgReconciliationDrifts.Load() }
