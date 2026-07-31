package HGKafkaPackage

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
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

func TestHGConsumerLagObserverAggregatesPartitionsByGroupAndTopic(t *testing.T) {
	hgResetKafkaMetricsForTest()
	observer := HGNewConsumerLagObserver("feed-reader", []string{"orders"})
	t.Cleanup(observer.Close)

	observer.ObserveFetch(kgo.Fetches{{Topics: []kgo.FetchTopic{{
		Topic: "orders",
		Partitions: []kgo.FetchPartition{
			{Partition: 0, HighWatermark: 110, Records: []*kgo.Record{{Topic: "orders", Partition: 0, Offset: 100}}},
			{Partition: 1, HighWatermark: 220, Records: []*kgo.Record{{Topic: "orders", Partition: 1, Offset: 200}}},
		},
	}}}})

	metrics := hgRenderKafkaMetrics(t)
	if !strings.Contains(metrics, `mlc_kafka_consumer_lag_records{group="feed-reader",topic="orders"} 30`) {
		t.Fatalf("metrics missing partition-aware lag: %s", metrics)
	}
	if strings.Contains(metrics, "partition=") {
		t.Fatalf("partition label must not be exported: %s", metrics)
	}
}

func TestHGConsumerLagObserverTracksProcessingOutcomes(t *testing.T) {
	hgResetKafkaMetricsForTest()
	observer := HGNewConsumerLagObserver("feed-reader", []string{"orders"})
	t.Cleanup(observer.Close)
	record := &kgo.Record{Topic: "orders", Partition: 0, Offset: 100}
	observer.ObserveFetch(kgo.Fetches{{Topics: []kgo.FetchTopic{{
		Topic:      "orders",
		Partitions: []kgo.FetchPartition{{Partition: 0, HighWatermark: 110, Records: []*kgo.Record{record}}},
	}}}})

	observer.ObserveRetryable(record)
	if metrics := hgRenderKafkaMetrics(t); !strings.Contains(metrics, `mlc_kafka_consumer_lag_records{group="feed-reader",topic="orders"} 10`) {
		t.Fatalf("retryable failure reduced lag: %s", metrics)
	}

	observer.ObserveSuccessful(record)
	if metrics := hgRenderKafkaMetrics(t); !strings.Contains(metrics, `mlc_kafka_consumer_lag_records{group="feed-reader",topic="orders"} 9`) {
		t.Fatalf("successful record did not reduce lag: %s", metrics)
	}

	terminal := &kgo.Record{Topic: "orders", Partition: 0, Offset: 101}
	observer.ObserveTerminal(terminal)
	if metrics := hgRenderKafkaMetrics(t); !strings.Contains(metrics, `mlc_kafka_consumer_lag_records{group="feed-reader",topic="orders"} 8`) {
		t.Fatalf("terminal record parked in DLQ did not reduce lag: %s", metrics)
	}
}

func TestHGConsumerLagObserverCloseRemovesMetrics(t *testing.T) {
	hgResetKafkaMetricsForTest()
	observer := HGNewConsumerLagObserver("feed-reader", []string{"orders"})
	observer.ObserveFetch(kgo.Fetches{{Topics: []kgo.FetchTopic{{
		Topic:      "orders",
		Partitions: []kgo.FetchPartition{{Partition: 0, HighWatermark: 110, Records: []*kgo.Record{{Topic: "orders", Partition: 0, Offset: 100}}}},
	}}}})
	observer.Close()

	if metrics := hgRenderKafkaMetrics(t); strings.Contains(metrics, `group="feed-reader"`) {
		t.Fatalf("closed observer remains exported: %s", metrics)
	}
}

func TestHGConsumerLagObserverRemovesRevokedPartitions(t *testing.T) {
	hgResetKafkaMetricsForTest()
	observer := HGNewConsumerLagObserver("feed-reader", []string{"orders"})
	t.Cleanup(observer.Close)
	observer.ObserveFetch(kgo.Fetches{{Topics: []kgo.FetchTopic{{
		Topic: "orders",
		Partitions: []kgo.FetchPartition{
			{Partition: 0, HighWatermark: 110, Records: []*kgo.Record{{Topic: "orders", Partition: 0, Offset: 100}}},
			{Partition: 1, HighWatermark: 220, Records: []*kgo.Record{{Topic: "orders", Partition: 1, Offset: 200}}},
		},
	}}}})

	observer.ObservePartitionsRevoked(map[string][]int32{"orders": {0}})

	if metrics := hgRenderKafkaMetrics(t); !strings.Contains(metrics, `mlc_kafka_consumer_lag_records{group="feed-reader",topic="orders"} 20`) {
		t.Fatalf("revoked partition remains in lag: %s", metrics)
	}
}

func hgRenderKafkaMetrics(t *testing.T) string {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	HGKafkaMetricsHandler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	return recorder.Body.String()
}
