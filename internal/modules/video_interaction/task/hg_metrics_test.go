package VideoInteractionTaskPackage

import (
	"bytes"
	"strings"
	"testing"
)

func TestHGWritePrometheusMetricsUsesOnlyFixedStreamLabels(t *testing.T) {
	var output bytes.Buffer
	HGWritePrometheusMetrics(&output)
	text := output.String()
	for _, stream := range hgProjectionStreams {
		if !strings.Contains(text, `stream="`+string(stream)+`"`) {
			t.Fatalf("metrics missing fixed stream %q: %s", stream, text)
		}
	}
	if strings.Count(text, "mlc_interaction_reproject_runs_total") != 4 {
		t.Fatalf("runs metric series must be exactly four: %s", text)
	}
}
