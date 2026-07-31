package CoinRepositoryPackage

import (
	"bytes"
	"strings"
	"testing"
)

func TestHGWritePrometheusMetricsUsesOnlyFixedOperationAndResultLabels(t *testing.T) {
	var output bytes.Buffer
	HGWritePrometheusMetrics(&output)
	text := output.String()
	if strings.Contains(text, "user_id") || strings.Contains(text, "request_id") || strings.Contains(text, "business_key") {
		t.Fatalf("high-cardinality label found:\n%s", text)
	}
	if !strings.Contains(text, `operation="debit",result="success"`) || !strings.Contains(text, `operation="refund",result="failure"`) {
		t.Fatalf("fixed labels missing:\n%s", text)
	}
}
