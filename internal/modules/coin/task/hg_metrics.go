package CoinTaskPackage

import (
	"fmt"
	"io"
	"sync/atomic"
)

var hgReconciliationDrifts atomic.Uint64

func hgObserveReconciliationDrift(count int) { hgReconciliationDrifts.Add(uint64(count)) }

// HGWritePrometheusMetrics 输出只检测对账的累计漂移数，不包含用户标签或自动修复结果。
func HGWritePrometheusMetrics(w io.Writer) {
	_, _ = fmt.Fprintf(w, "mlc_coin_reconciliation_drifts_total %d\n", hgReconciliationDrifts.Load())
}

// HGReconciliationMetricsSnapshot 返回无用户维度的累计漂移快照，供受鉴权运维 API 展示。
func HGReconciliationMetricsSnapshot() uint64 { return hgReconciliationDrifts.Load() }
