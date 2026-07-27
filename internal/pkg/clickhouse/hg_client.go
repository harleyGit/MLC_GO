package clickhouse

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const hgMaxClickHouseErrorBody = 4096

var hgClickHouseIdentifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// HGConfig 描述 ClickHouse HTTP 客户端连接与统计表配置。
type HGConfig struct {
	Endpoint             string
	Database             string
	Username             string
	Password             string
	StatisticEventsTable string
	StatisticTotalsTable string
	WriteTimeout         time.Duration
	QueryTimeout         time.Duration
}

// HGStatisticEvent 是 ClickHouse 保存的统计权威事件行。
type HGStatisticEvent struct {
	EventID           string `json:"event_id"`
	EventName         string `json:"event_name"`
	EventKey          string `json:"event_key"`
	SubmissionID      string `json:"submission_id"`
	UserID            string `json:"user_id"`
	EventVersion      int    `json:"event_version"`
	EventTimestamp    int64  `json:"event_timestamp"`
	SourceService     string `json:"source_service"`
	TraceID           string `json:"trace_id"`
	RequestID         string `json:"request_id"`
	KafkaTopic        string `json:"kafka_topic"`
	KafkaPartition    int32  `json:"kafka_partition"`
	KafkaOffset       int64  `json:"kafka_offset"`
	RedisGeneration   string `json:"redis_generation"`
	RedisShard        int    `json:"redis_shard"`
	Payload           string `json:"payload"`
	IngestedTimestamp int64  `json:"ingested_timestamp"`
}

// HGStatisticDimension 标识一个 Redis shard 内的事件计数维度。
type HGStatisticDimension struct {
	Shard     int
	EventName string
}

// HGClient 使用 ClickHouse HTTP API 写入权威事件并读取精确聚合。
type HGClient struct {
	config HGConfig
	client *http.Client
}

// NewHGClient 创建长期复用的 ClickHouse HTTP 客户端。
func NewHGClient(config HGConfig) (*HGClient, error) {
	config.Endpoint = strings.TrimRight(strings.TrimSpace(config.Endpoint), "/")
	config.Database = strings.TrimSpace(config.Database)
	config.Username = strings.TrimSpace(config.Username)
	if config.Endpoint == "" || config.Database == "" || config.Username == "" {
		return nil, fmt.Errorf("clickhouse endpoint, database and username cannot be empty")
	}
	if _, err := url.ParseRequestURI(config.Endpoint); err != nil {
		return nil, fmt.Errorf("invalid clickhouse endpoint: %w", err)
	}
	for name, identifier := range map[string]string{
		"database":               config.Database,
		"statistic events table": config.StatisticEventsTable,
		"statistic totals table": config.StatisticTotalsTable,
	} {
		if identifier != "" && !hgClickHouseIdentifierPattern.MatchString(identifier) {
			return nil, fmt.Errorf("invalid clickhouse %s", name)
		}
	}
	if config.WriteTimeout <= 0 || config.QueryTimeout <= 0 {
		return nil, fmt.Errorf("clickhouse timeouts must be positive")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 50
	transport.MaxIdleConnsPerHost = 50
	return &HGClient{config: config, client: &http.Client{Transport: transport}}, nil
}

// PingContext 验证 ClickHouse HTTP endpoint 可用。
func (c *HGClient) PingContext(ctx context.Context) error {
	return c.execute(ctx, c.config.QueryTimeout, "SELECT 1", nil, nil)
}

// StoreStatisticEvent 使用等待确认的 async insert 保存权威事件。
func (c *HGClient) StoreStatisticEvent(ctx context.Context, event HGStatisticEvent) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("clickhouse client cannot be nil")
	}
	if c.config.StatisticEventsTable == "" {
		return fmt.Errorf("clickhouse statistic events table cannot be empty")
	}
	if event.IngestedTimestamp == 0 {
		event.IngestedTimestamp = time.Now().UTC().UnixMilli()
	}
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal statistic event: %w", err)
	}
	query := fmt.Sprintf("INSERT INTO %s.%s SETTINGS async_insert=1, wait_for_async_insert=1 FORMAT JSONEachRow", c.config.Database, c.config.StatisticEventsTable)
	return c.execute(ctx, c.config.WriteTimeout, query, append(body, '\n'), nil)
}

// GetStatisticTotals 读取按 EventID 精确去重后的 shard 事件累计值。
func (c *HGClient) GetStatisticTotals(ctx context.Context, generation string) (map[HGStatisticDimension]uint64, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("clickhouse client cannot be nil")
	}
	if c.config.StatisticTotalsTable == "" {
		return nil, fmt.Errorf("clickhouse statistic totals table cannot be empty")
	}
	query := fmt.Sprintf("SELECT redis_shard, event_name, toString(uniqExactMerge(event_ids)) AS total FROM %s.%s WHERE redis_generation = {generation:String} GROUP BY redis_shard, event_name FORMAT JSONEachRow", c.config.Database, c.config.StatisticTotalsTable)
	var response bytes.Buffer
	if err := c.execute(ctx, c.config.QueryTimeout, query, nil, map[string]string{"param_generation": generation}, &response); err != nil {
		return nil, err
	}
	totals := make(map[HGStatisticDimension]uint64)
	decoder := json.NewDecoder(&response)
	for decoder.More() {
		var row struct {
			Shard     int    `json:"redis_shard"`
			EventName string `json:"event_name"`
			Total     string `json:"total"`
		}
		if err := decoder.Decode(&row); err != nil {
			return nil, fmt.Errorf("decode clickhouse statistic totals: %w", err)
		}
		total, err := strconv.ParseUint(row.Total, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse clickhouse statistic total: %w", err)
		}
		totals[HGStatisticDimension{Shard: row.Shard, EventName: row.EventName}] = total
	}
	return totals, nil
}

// Close 释放 HTTP idle connections。
func (c *HGClient) Close() error {
	if c != nil && c.client != nil {
		c.client.CloseIdleConnections()
	}
	return nil
}

func (c *HGClient) execute(ctx context.Context, timeout time.Duration, query string, body []byte, params map[string]string, outputs ...io.Writer) error {
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	endpoint, err := url.Parse(c.config.Endpoint)
	if err != nil {
		return fmt.Errorf("parse clickhouse endpoint: %w", err)
	}
	values := endpoint.Query()
	values.Set("query", query)
	for key, value := range params {
		values.Set(key, value)
	}
	endpoint.RawQuery = values.Encode()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build clickhouse request: %w", err)
	}
	request.SetBasicAuth(c.config.Username, c.config.Password)
	request.Header.Set("Content-Type", "application/x-ndjson")
	response, err := c.client.Do(request)
	if err != nil {
		return fmt.Errorf("execute clickhouse request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		message, _ := io.ReadAll(io.LimitReader(response.Body, hgMaxClickHouseErrorBody))
		return fmt.Errorf("clickhouse returned status %d: %s", response.StatusCode, strings.TrimSpace(string(message)))
	}
	if len(outputs) > 0 && outputs[0] != nil {
		if _, err := io.Copy(outputs[0], response.Body); err != nil {
			return fmt.Errorf("read clickhouse response: %w", err)
		}
	}
	return nil
}
