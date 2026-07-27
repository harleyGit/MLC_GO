package statistic

import (
	"bytes"
	"strings"
	"testing"
)

func TestHGWritePrometheusMetricsExposesAuthorityAndReconcileState(t *testing.T) {
	hgResetStatisticMetricsForTest()
	hgStatisticAuthorityWrites.Add(2)
	hgStatisticReconcileRuns.Add(1)
	hgStatisticReconcileCurrentDrift.Store(3)
	var output bytes.Buffer
	HGWritePrometheusMetrics(&output)
	for _, metric := range []string{
		"mlc_statistic_clickhouse_writes_total 2",
		"mlc_statistic_reconcile_runs_total 1",
		"mlc_statistic_reconcile_current_drift 3",
	} {
		if !strings.Contains(output.String(), metric) {
			t.Fatalf("metrics missing %q: %s", metric, output.String())
		}
	}
}
