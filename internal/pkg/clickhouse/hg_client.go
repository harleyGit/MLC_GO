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
	DanmakuHistoryTable  string
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

// HGDanmakuHistory 是按视频时间线保存的 ClickHouse 历史行。
type HGDanmakuHistory struct {
	DanmakuID, SubmissionID, VideoID, UserID, RequestID, Content, Mode, Color string
	ProgressMS                                                                uint32
	FontSize                                                                  uint8
	CreatedAt                                                                 int64
	KafkaTopic                                                                string
	KafkaPartition                                                            int32
	KafkaOffset                                                               int64
}

// HGDanmakuCursor 是 ClickHouse 视频时间线的稳定复合游标。
type HGDanmakuCursor struct {
	ProgressMS uint32
	CreatedAt  int64
	DanmakuID  string
}

// HGClient 使用 ClickHouse HTTP API 写入权威事件并读取精确聚合。
type HGClient struct {
	config HGConfig
	client *http.Client
}

// NewHGClient 创建长期复用的 ClickHouse HTTP 客户端。
func NewHGClient(config HGConfig) (*HGClient, error) {

	// TrimRight、TrimSpace 清理 Endpoint 两边空格，并去掉末尾 /。
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
		"danmaku history table":  config.DanmakuHistoryTable,
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

// StoreDanmakuHistory 使用单个 JSONEachRow 请求批量写入同一 Kafka 分区的有界弹幕批次。
func (c *HGClient) StoreDanmakuHistory(ctx context.Context, items []HGDanmakuHistory) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("clickhouse client cannot be nil")
	}
	if c.config.DanmakuHistoryTable == "" {
		return fmt.Errorf("clickhouse danmaku history table cannot be empty")
	}
	if len(items) == 0 {
		return nil
	}
	var body bytes.Buffer
	encoder := json.NewEncoder(&body)
	for _, item := range items {
		row := struct {
			DanmakuID      string `json:"danmaku_id"`
			SubmissionID   string `json:"submission_id"`
			VideoID        string `json:"video_id"`
			UserID         string `json:"user_id"`
			RequestID      string `json:"request_id"`
			Content        string `json:"content"`
			ProgressMS     uint32 `json:"progress_ms"`
			Mode           string `json:"mode"`
			Color          string `json:"color"`
			FontSize       uint8  `json:"font_size"`
			CreatedAt      int64  `json:"created_at"`
			KafkaTopic     string `json:"kafka_topic"`
			KafkaPartition int32  `json:"kafka_partition"`
			KafkaOffset    int64  `json:"kafka_offset"`
			IngestedAt     int64  `json:"ingested_at"`
		}{item.DanmakuID, item.SubmissionID, item.VideoID, item.UserID, item.RequestID, item.Content, item.ProgressMS, item.Mode, item.Color, item.FontSize, item.CreatedAt, item.KafkaTopic, item.KafkaPartition, item.KafkaOffset, time.Now().UTC().UnixMilli()}
		if err := encoder.Encode(row); err != nil {
			return fmt.Errorf("encode danmaku history: %w", err)
		}
	}
	query := fmt.Sprintf("INSERT INTO %s.%s SETTINGS async_insert=1, wait_for_async_insert=1 FORMAT JSONEachRow", c.config.Database, c.config.DanmakuHistoryTable)
	return c.execute(ctx, c.config.WriteTimeout, query, body.Bytes(), nil)
}

// ListDanmakuHistory 按视频、播放时间窗和复合游标读取历史，禁止 OFFSET 深分页。
func (c *HGClient) ListDanmakuHistory(ctx context.Context, videoID string, fromMS, toMS uint32, cursor HGDanmakuCursor, limit int) ([]HGDanmakuHistory, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("clickhouse client cannot be nil")
	}
	if c.config.DanmakuHistoryTable == "" || strings.TrimSpace(videoID) == "" {
		return nil, fmt.Errorf("clickhouse danmaku history query is invalid")
	}
	if toMS <= fromMS || toMS-fromMS > 300_000 || limit < 1 || limit > 1000 {
		return nil, fmt.Errorf("clickhouse danmaku history bounds are invalid")
	}
	query := fmt.Sprintf(`SELECT danmaku_id, submission_id, video_id, user_id, request_id, content, progress_ms, mode, color, font_size, created_at, kafka_topic, kafka_partition, kafka_offset
FROM %s.%s
WHERE video_id = {video_id:String}
  AND progress_ms >= {from_ms:UInt32}
  AND progress_ms < {to_ms:UInt32}
  AND (progress_ms, created_at, danmaku_id) > ({cursor_progress:UInt32}, {cursor_created:Int64}, {cursor_id:String})
ORDER BY progress_ms, created_at, danmaku_id
LIMIT {limit:UInt32}
FORMAT JSONEachRow`, c.config.Database, c.config.DanmakuHistoryTable)
	params := map[string]string{
		"param_video_id":        videoID,
		"param_from_ms":         strconv.FormatUint(uint64(fromMS), 10),
		"param_to_ms":           strconv.FormatUint(uint64(toMS), 10),
		"param_cursor_progress": strconv.FormatUint(uint64(cursor.ProgressMS), 10),
		"param_cursor_created":  strconv.FormatInt(cursor.CreatedAt, 10),
		"param_cursor_id":       cursor.DanmakuID,
		"param_limit":           strconv.Itoa(limit),
	}
	var response bytes.Buffer
	if err := c.execute(ctx, c.config.QueryTimeout, query, nil, params, &response); err != nil {
		return nil, err
	}
	items := make([]HGDanmakuHistory, 0, limit)
	decoder := json.NewDecoder(&response)
	for decoder.More() {
		var row struct {
			DanmakuID      string `json:"danmaku_id"`
			SubmissionID   string `json:"submission_id"`
			VideoID        string `json:"video_id"`
			UserID         string `json:"user_id"`
			RequestID      string `json:"request_id"`
			Content        string `json:"content"`
			ProgressMS     uint32 `json:"progress_ms"`
			Mode           string `json:"mode"`
			Color          string `json:"color"`
			FontSize       uint8  `json:"font_size"`
			CreatedAt      int64  `json:"created_at"`
			KafkaTopic     string `json:"kafka_topic"`
			KafkaPartition int32  `json:"kafka_partition"`
			KafkaOffset    int64  `json:"kafka_offset"`
		}
		if err := decoder.Decode(&row); err != nil {
			return nil, fmt.Errorf("decode clickhouse danmaku history: %w", err)
		}
		items = append(items, HGDanmakuHistory{
			DanmakuID: row.DanmakuID, SubmissionID: row.SubmissionID, VideoID: row.VideoID, UserID: row.UserID,
			RequestID: row.RequestID, Content: row.Content, ProgressMS: row.ProgressMS, Mode: row.Mode,
			Color: row.Color, FontSize: row.FontSize, CreatedAt: row.CreatedAt, KafkaTopic: row.KafkaTopic,
			KafkaPartition: row.KafkaPartition, KafkaOffset: row.KafkaOffset,
		})
	}
	return items, nil
}

// PingContext 验证 ClickHouse HTTP endpoint 可用。真正执行http请求
// PingContext 用来探测 ClickHouse 服务是否存活，底层复用通用的execute函数发送 SELECT 1
func (c *HGClient) PingContext(ctx context.Context) error {
	/**
	- Ping 逻辑：发送`SELECT 1`测试连通性。
	- 把上层 ctx 透传给 execute，同时传入内部 timeout。
	- body、params、outputs 全部 nil，只探测连通，不需要返回数据
	*/
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

		// CloseIdleConnections 关闭当前 HTTP Transport 中处于 idle（空闲）状态的连接。
		c.client.CloseIdleConnections()
	}
	return nil
}

// execute  是通用 HTTP 请求执行函数，负责构造 HTTP 请求、带超时调用 ClickHouse HTTP 接口、处理响应与错误
//	@param ctx 上层传入父上下文（Ping 的时候就是 pingCtx，可以携带外部超时、取消信号）
//	@param timeout 函数内部二次超时时间 c.config.QueryTimeout
//	@param query clickhouse SQL 语句，ping 场景是 SELECT 1
//	@param body  POST 请求 body，ping 场景 nil；批量 ndjson 查询时填充数据
//	@param params HTTP query 额外参数
//	@param outputs 可变参数，把 clickhouse 返回流写入 writer；ping 不需要接收输出传 nil
//	@return error 
func (c *HGClient) execute(ctx context.Context, timeout time.Duration, query string, body []byte, params map[string]string, outputs ...io.Writer) error {
	// 基于**父 ctx**再包装一层超时`timeout`，得到`requestCtx`给 http 请求使用。
	// 继承父 ctx 的取消信号：如果上层`pingCtx`提前超时 /cancel，`requestCtx`会跟着取消；同时内部又有自己的`timeout`兜底
	// 超时逻辑：**哪个先到就哪个生效**
	// 上层 ctx 超时时间 `clickHouseConfig.QueryTimeoutDuration`，内部 timeout `c.config.QueryTimeout`
	// 如果两个配置不一致，取 更小的那个 触发取消。
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	// 解析配置的 ClickHouse 地址（例如 `http://127.0.0.1:8123`）
	endpoint, err := url.Parse(c.config.Endpoint)
	if err != nil {
		// ，`%w` Go1.13 错误包装，保留原始 error，上层可以用`errors.Is`做判断
		return fmt.Errorf("parse clickhouse endpoint: %w", err)
	}
	// 获取 URL 现有的 query 参数
	values := endpoint.Query()
	// 设置`query`参数为 SQL 语句（ClickHouse HTTP 协议约定）
	values.Set("query", query)
	// 追加自定义 params（例如`database`、`default_format`等 clickHouse http 参数）
	for key, value := range params {
		values.Set(key, value)
	}
	// Encode 生成编码后的 query 字符串赋值回 UR，示例：http://127.0.0.1:8123?query=SELECT+1
	endpoint.RawQuery = values.Encode()
	// http.NewRequestWithContext：绑定 requestCtx 到 http 请求，context 一旦 Done，`c.client.Do(request)`就会中断 http 调用
	// method: POST；body 用`bytes.NewReader(body)`，ping 场景 body=nil，请求无 body
	request, err := http.NewRequestWithContext(requestCtx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build clickhouse request: %w", err)
	}
	// BasicAuth：ClickHouse http 接口账号密码认证。
	request.SetBasicAuth(c.config.Username, c.config.Password)
	// Content-Type 固定`application/x-ndjson`：适合 ndjson 格式写入；普通 SQL 查询其实不需要这个头，Ping `SELECT 1`下这个 header 是多余的，但不影响 ClickHouse 执行
	request.Header.Set("Content-Type", "application/x-ndjson")
	// c.client 是 *http.Client，要注意：http.Client 不要每次新建，全局复用；Transport 要合理配置 MaxIdleConns 等。
	// client.Do 阻塞发起网络请求；如果requestCtx超时 / 取消，这里直接返回 context 相关错误 (`context.DeadlineExceeded` / `context.Canceled`)。
	response, err := c.client.Do(request)
	if err != nil {
		return fmt.Errorf("execute clickhouse request: %w", err)
	}
	// 强制必须写，否则 http 连接无法归还连接池，连接泄漏。即使报错也要 close
	defer response.Body.Close()
	// 判断 HTTP 状态码：非 2xx 全部视为错误
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		// `io.LimitReader`：限制读取错误返回 body 最大字节`hgMaxClickHouseErrorBody`，防止报错返回超大响应体吃掉内存；`_`忽略读取错误，尽力拿错误信息
		message, _ := io.ReadAll(io.LimitReader(response.Body, hgMaxClickHouseErrorBody))
		return fmt.Errorf("clickhouse returned status %d: %s", response.StatusCode, strings.TrimSpace(string(message)))
	}
	/**
	- 如果传入了`io.Writer`，把 response.Body 流拷贝写入 writer（比如文件、buffer）。
	- Ping 调用时 outputs 传 nil，不走拷贝逻辑；直接 return nil 成功。
	- Ping 场景下 ClickHouse 收到`SELECT 1` HTTP 请求返回 `1\n`，状态码 200，就代表服务存活
	*/
	if len(outputs) > 0 && outputs[0] != nil {
		if _, err := io.Copy(outputs[0], response.Body); err != nil {
			return fmt.Errorf("read clickhouse response: %w", err)
		}
	}
	return nil
}
