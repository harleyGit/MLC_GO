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

func TestHGClientStoresDanmakuHistoryAsBatch(t *testing.T) {
	var query string
	var body []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.Query().Get("query")
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	client, err := NewHGClient(HGConfig{Endpoint: server.URL, Database: "mlc", Username: "app", DanmakuHistoryTable: "video_danmaku_history", WriteTimeout: time.Second, QueryTimeout: time.Second})
	if err != nil {
		t.Fatalf("NewHGClient() error = %v", err)
	}
	items := []HGDanmakuHistory{
		{DanmakuID: "DMK_1", VideoID: "video-1", UserID: "user-1", Content: "one", CreatedAt: 1720000000001},
		{DanmakuID: "DMK_2", VideoID: "video-1", UserID: "user-2", Content: "two", CreatedAt: 1720000000002},
	}
	if err := client.StoreDanmakuHistory(context.Background(), items); err != nil {
		t.Fatalf("StoreDanmakuHistory() error = %v", err)
	}
	if !strings.Contains(query, "INSERT INTO mlc.video_danmaku_history") || !strings.Contains(query, "FORMAT JSONEachRow") {
		t.Fatalf("query = %q", query)
	}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], `"danmaku_id":"DMK_1"`) || !strings.Contains(lines[1], `"danmaku_id":"DMK_2"`) {
		t.Fatalf("body = %q", body)
	}
}

func TestHGClientListsDanmakuHistoryWithKeysetBounds(t *testing.T) {
	var query string
	var params map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.Query().Get("query")
		params = make(map[string]string)
		for key, value := range r.URL.Query() {
			if strings.HasPrefix(key, "param_") && len(value) > 0 {
				params[key] = value[0]
			}
		}
		_, _ = io.WriteString(w, `{"danmaku_id":"DMK_2","video_id":"video-1","user_id":"user-2","content":"two","progress_ms":1200,"mode":"scroll","color":"#FFFFFF","font_size":25,"created_at":1720000000002,"kafka_topic":"danmaku","kafka_partition":3,"kafka_offset":8}`+"\n")
	}))
	defer server.Close()
	client, err := NewHGClient(HGConfig{Endpoint: server.URL, Database: "mlc", Username: "app", DanmakuHistoryTable: "video_danmaku_history", WriteTimeout: time.Second, QueryTimeout: time.Second})
	if err != nil {
		t.Fatalf("NewHGClient() error = %v", err)
	}
	items, err := client.ListDanmakuHistory(context.Background(), "video-1", 1000, 2000, HGDanmakuCursor{ProgressMS: 1100, CreatedAt: 1720000000001, DanmakuID: "DMK_1"}, 200)
	if err != nil {
		t.Fatalf("ListDanmakuHistory() error = %v", err)
	}
	if len(items) != 1 || items[0].DanmakuID != "DMK_2" || !strings.Contains(query, "(progress_ms, created_at, danmaku_id) >") || strings.Contains(strings.ToUpper(query), " OFFSET ") {
		t.Fatalf("items=%#v query=%q", items, query)
	}
	if params["param_video_id"] != "video-1" || params["param_limit"] != "200" {
		t.Fatalf("params = %#v", params)
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
