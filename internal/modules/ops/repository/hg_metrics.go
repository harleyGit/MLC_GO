package OpsRepositoryPackage

import (
	"fmt"
	"io"
	"strings"
	"sync/atomic"
)

var (
	hgAssetAuditRecoveryConflicts atomic.Uint64
	hgAssetAuditAPIConflicts      atomic.Uint64
)

func hgObserveAssetAuditEventKeyConflict(sourceIP string) {
	// The recovery worker writes this reserved source value; every external request is deliberately collapsed into the fixed ops_api bucket.
	if strings.TrimSpace(sourceIP) == "system:correction-recovery" {
		hgAssetAuditRecoveryConflicts.Add(1)
		return
	}
	hgAssetAuditAPIConflicts.Add(1)
}

// HGWritePrometheusMetrics exposes only fixed audit writer sources; event keys and request IDs must never become labels.
// Counters are process-local and monotonic, so deployment analysis must use Prometheus rate/increase functions across instance restarts.
func HGWritePrometheusMetrics(w io.Writer) {
	_, _ = fmt.Fprintf(w, "mlc_ops_asset_audit_event_key_conflicts_total{source=\"correction_recovery\"} %d\n", hgAssetAuditRecoveryConflicts.Load())
	_, _ = fmt.Fprintf(w, "mlc_ops_asset_audit_event_key_conflicts_total{source=\"ops_api\"} %d\n", hgAssetAuditAPIConflicts.Load())
}
