package clickhouse

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHGClientStoresStatisticEventAsJSONEachRow(t *testing.T) {
	var query string
	var body []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.Query().Get("query")
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewHGClient(HGConfig{Endpoint: server.URL, Database: "mlc", Username: "app", Password: "secret", StatisticEventsTable: "statistic_events", WriteTimeout: time.Second, QueryTimeout: time.Second})
	if err != nil {
		t.Fatalf("NewHGClient() error = %v", err)
	}
	event := HGStatisticEvent{
		EventID: "event-1", EventName: "video.published", EventKey: "submission-1",
		SubmissionID: "submission-1", UserID: "user-1", EventVersion: 1,
		EventTimestamp: 1720000000123, SourceService: "mlc-go", KafkaTopic: "mlc.domain.events",
		KafkaPartition: 3, KafkaOffset: 11, RedisGeneration: "v2", RedisShard: 3,
		Payload: `{"submissionId":"submission-1","userId":"user-1"}`,
	}
	if err := client.StoreStatisticEvent(context.Background(), event); err != nil {
		t.Fatalf("StoreStatisticEvent() error = %v", err)
	}
	if !strings.Contains(query, "INSERT INTO mlc.statistic_events") || !strings.Contains(query, "FORMAT JSONEachRow") || !strings.Contains(query, "wait_for_async_insert=1") {
		t.Fatalf("query = %q, want durable JSONEachRow insert", query)
	}
	var row map[string]any
	if err := json.Unmarshal(body, &row); err != nil {
		t.Fatalf("insert body is not JSON: %v", err)
	}
	if row["event_id"] != "event-1" || row["submission_id"] != "submission-1" || row["redis_generation"] != "v2" {
		t.Fatalf("insert row = %#v", row)
	}
}

func TestHGClientReadsExactStatisticTotals(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "{\"redis_shard\":3,\"event_name\":\"video.published\",\"total\":\"12\"}\n")
	}))
	defer server.Close()

	client, err := NewHGClient(HGConfig{Endpoint: server.URL, Database: "mlc", Username: "app", StatisticTotalsTable: "statistic_event_totals", WriteTimeout: time.Second, QueryTimeout: time.Second})
	if err != nil {
		t.Fatalf("NewHGClient() error = %v", err)
	}
	totals, err := client.GetStatisticTotals(context.Background(), "v2")
	if err != nil {
		t.Fatalf("GetStatisticTotals() error = %v", err)
	}
	if totals[HGStatisticDimension{Shard: 3, EventName: "video.published"}] != 12 {
		t.Fatalf("totals = %#v, want shard total 12", totals)
	}
}

func TestHGClientDoesNotLeakPasswordInServerErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, strings.Repeat("x", 10000), http.StatusInternalServerError)
	}))
	defer server.Close()

	client, err := NewHGClient(HGConfig{Endpoint: server.URL, Database: "mlc", Username: "app", Password: "top-secret", StatisticEventsTable: "statistic_events", WriteTimeout: time.Second, QueryTimeout: time.Second})
	if err != nil {
		t.Fatalf("NewHGClient() error = %v", err)
	}
	err = client.StoreStatisticEvent(context.Background(), HGStatisticEvent{EventID: "event-1", EventName: "video.published", Payload: `{}`})
	if err == nil {
		t.Fatal("StoreStatisticEvent() error = nil, want server error")
	}
	if strings.Contains(err.Error(), "top-secret") || len(err.Error()) > 5000 {
		t.Fatalf("unsafe error = %q", err)
	}
}
