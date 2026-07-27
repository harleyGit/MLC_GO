package HGKafkaPackage

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHGKafkaMetricsHandlerAppendsComponentMetrics(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	HGKafkaMetricsHandler(func(w io.Writer) { _, _ = fmt.Fprint(w, "mlc_component_metric 1\n") }).ServeHTTP(recorder, request)
	if !strings.Contains(recorder.Body.String(), "mlc_component_metric 1") {
		t.Fatalf("metrics = %s", recorder.Body.String())
	}
}

func TestHGKafkaMetricsHandlerExposesConsumerMetrics(t *testing.T) {
	hgResetKafkaMetricsForTest()
	hgObserveConsumeBatch(12, 25*time.Millisecond)
	hgObserveCommit(2, 5*time.Millisecond, nil)
	hgKafkaHandlerFailures.Add(1)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	HGKafkaMetricsHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	for _, metric := range []string{
		"# TYPE mlc_kafka_consume_batches_total counter",
		"mlc_kafka_consume_batches_total 1",
		"mlc_kafka_consume_batch_records_total 12",
		"mlc_kafka_commits_total 1",
		"mlc_kafka_handler_failures_total 1",
		"# TYPE mlc_kafka_assigned_partitions gauge",
	} {
		if !strings.Contains(recorder.Body.String(), metric) {
			t.Fatalf("metrics missing %q: %s", metric, recorder.Body.String())
		}
	}
}

func TestHGKafkaMetricsHandlerRejectsNonGET(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/metrics", nil)
	HGKafkaMetricsHandler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", recorder.Code)
	}
}
