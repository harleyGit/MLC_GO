package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

const (
	// endpoint 固定为 Bilibili API 域名，构造阶段还会再次校验 scheme/host，避免配置形成 SSRF 入口。
	hgBilibiliRecommendEndpoint = "https://api.bilibili.com/x/web-interface/wbi/index/top/feed/rcmd"
	// 响应上限防止第三方异常响应导致进程内存被无界占用。
	hgBilibiliMaxResponseBytes = 2 << 20
)

// HGHTTPDoer 抽象标准库 HTTP Client，便于替换 transport 和单元测试。
type HGHTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// HGBilibiliConfig 定义 Bilibili 推荐接口的有界请求参数。
// 所有值都会在构造阶段归一化，避免运行时接受无上限重试、返回数量或请求频率。
type HGBilibiliConfig struct {
	Endpoint       string        // 仅允许 https://api.bilibili.com 下的地址。
	UserAgent      string        // 标识本服务来源，不包含用户身份或 Cookie。
	RequestTimeout time.Duration // 每次 HTTP 尝试的超时，不是整个任务的总超时。
	MaxItems       int           // 单批最多接受 50 条，默认 12 条。
	RetryCount     int           // 额外重试次数，最大 3 次。
	RatePerSecond  float64       // 进程内令牌桶速率，最大 1 req/s。
}

// HGBilibiliPlatform 抓取 Bilibili 首页推荐接口中的公开视频元数据。
// limiter 只提供单进程限流；多副本部署仍需 CronJob Forbid 或 Redis lease 控制全局请求频率。
type HGBilibiliPlatform struct {
	client  HGHTTPDoer
	config  HGBilibiliConfig
	limiter *rate.Limiter
}

// hgBilibiliEnvelope 只声明当前业务需要的顶层协议字段；未知字段由 encoding/json 自动忽略。
type hgBilibiliEnvelope struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Items []hgBilibiliItem `json:"item"`
	} `json:"data"`
}

// hgBilibiliItem 是上游协议 DTO，不直接作为管理 API 响应，避免平台字段变化污染统一业务模型。
type hgBilibiliItem struct {
	BVID     string `json:"bvid"`
	Goto     string `json:"goto"`
	URI      string `json:"uri"`
	Picture  string `json:"pic"`
	Title    string `json:"title"`
	Duration int64  `json:"duration"`
	Pubdate  int64  `json:"pubdate"`
	Owner    struct {
		MID  int64  `json:"mid"`
		Name string `json:"name"`
	} `json:"owner"`
	Stat struct {
		View    int64 `json:"view"`
		Like    int64 `json:"like"`
		Danmaku int64 `json:"danmaku"`
	} `json:"stat"`
}

// NewHGBilibiliPlatform 创建带连接复用、超时、限流和有限重试的 Bilibili 数据源。
// client 为空时复用标准库 Transport 的安全默认值并调整连接池；测试可注入 HGHTTPDoer。
func NewHGBilibiliPlatform(client HGHTTPDoer, config HGBilibiliConfig) (*HGBilibiliPlatform, error) {
	if config.Endpoint == "" {
		config.Endpoint = hgBilibiliRecommendEndpoint
	}
	endpoint, err := url.Parse(config.Endpoint)
	if err != nil || endpoint.Scheme != "https" || endpoint.Hostname() != "api.bilibili.com" {
		return nil, errors.New("bilibili endpoint must use https://api.bilibili.com")
	}
	if client == nil {
		// Clone 避免直接修改全局 http.DefaultTransport，防止影响同进程其他 HTTP 客户端。
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.MaxIdleConns = 32
		transport.MaxIdleConnsPerHost = 8
		transport.IdleConnTimeout = 90 * time.Second
		transport.ResponseHeaderTimeout = 5 * time.Second
		client = &http.Client{Transport: transport}
	}
	if config.RequestTimeout <= 0 {
		config.RequestTimeout = 5 * time.Second
	}
	if config.MaxItems <= 0 || config.MaxItems > 50 {
		config.MaxItems = 12
	}
	if config.RetryCount < 0 || config.RetryCount > 3 {
		config.RetryCount = 2
	}
	if config.UserAgent == "" {
		config.UserAgent = "MLC_GO-HGCrawler/1.0"
	}
	if config.RatePerSecond <= 0 || config.RatePerSecond > 1 {
		config.RatePerSecond = 0.2
	}
	return &HGBilibiliPlatform{
		client:  client,
		config:  config,
		limiter: rate.NewLimiter(rate.Limit(config.RatePerSecond), 1),
	}, nil
}

// Name 返回稳定的平台标识。
func (p *HGBilibiliPlatform) Name() string { return "bilibili" }

// FetchRecommendations 获取并标准化 Bilibili 首页推荐视频。
// 仅网络错误、读取错误、429 和网关类 5xx 可以重试；协议错误和业务拒绝码立即失败，避免放大请求。
func (p *HGBilibiliPlatform) FetchRecommendations(ctx context.Context) ([]HGRecommendation, error) {
	if err := p.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("waiting bilibili rate limiter: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= p.config.RetryCount; attempt++ {
		items, retry, err := p.fetchOnce(ctx)
		if err == nil {
			return items, nil
		}
		lastErr = err
		if !retry || attempt == p.config.RetryCount {
			break
		}
		// 指数退避叠加小范围 jitter，降低多个实例同时恢复时再次冲击上游的概率。
		delay := time.Duration(300*(1<<attempt)+rand.IntN(200)) * time.Millisecond
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return nil, lastErr
}

// fetchOnce 执行一次 HTTP 尝试。
// 第二个返回值仅表示该错误是否允许由上层重试，不代表本次请求成功或数据是否为空。
func (p *HGBilibiliPlatform) fetchOnce(ctx context.Context) ([]HGRecommendation, bool, error) {
	requestCtx, cancel := context.WithTimeout(ctx, p.config.RequestTimeout)
	defer cancel()

	endpoint, _ := url.Parse(p.config.Endpoint)
	query := endpoint.Query()
	query.Set("fresh_type", "3")
	query.Set("ps", strconv.Itoa(p.config.MaxItems))
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, false, fmt.Errorf("building bilibili request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Referer", "https://www.bilibili.com/")
	req.Header.Set("User-Agent", p.config.UserAgent)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, true, fmt.Errorf("requesting bilibili recommendations: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		// 错误响应只丢弃最多 4 KiB，不把可能包含风控信息或身份信息的响应体写入错误和日志。
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		retry := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusBadGateway
		return nil, retry, fmt.Errorf("bilibili returned HTTP %d", resp.StatusCode)
	}

	// 多读 1 字节用于准确判断响应是否超过上限，而不是静默截断后继续解析不完整 JSON。
	limitedBody := io.LimitReader(resp.Body, hgBilibiliMaxResponseBytes+1)
	body, err := io.ReadAll(limitedBody)
	if err != nil {
		return nil, true, fmt.Errorf("reading bilibili response: %w", err)
	}
	if len(body) > hgBilibiliMaxResponseBytes {
		return nil, false, errors.New("bilibili response exceeds size limit")
	}
	var envelope hgBilibiliEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, false, fmt.Errorf("decoding bilibili response: %w", err)
	}
	if envelope.Code != 0 {
		return nil, false, fmt.Errorf("bilibili business code %d: %s", envelope.Code, envelope.Message)
	}

	items := make([]HGRecommendation, 0, min(len(envelope.Data.Items), p.config.MaxItems))
	seen := make(map[string]struct{}, len(envelope.Data.Items))
	for _, item := range envelope.Data.Items {
		// 推荐流可能混有直播、番剧或广告；第一版只接收 goto=av 且具备稳定 BVID/标题的普通视频。
		if item.Goto != "av" || item.BVID == "" || strings.TrimSpace(item.Title) == "" {
			continue
		}
		if _, exists := seen[item.BVID]; exists {
			continue
		}
		seen[item.BVID] = struct{}{}
		items = append(items, HGRecommendation{
			Platform:     p.Name(),
			ContentID:    item.BVID,
			Title:        strings.TrimSpace(item.Title),
			AuthorID:     strconv.FormatInt(item.Owner.MID, 10),
			AuthorName:   item.Owner.Name,
			CoverURL:     hgNormalizeHTTPS(item.Picture),
			TargetURL:    item.URI,
			Duration:     item.Duration,
			ViewCount:    item.Stat.View,
			LikeCount:    item.Stat.Like,
			CommentCount: item.Stat.Danmaku,
			PublishedAt:  time.Unix(item.Pubdate, 0).UTC(),
		})
		if len(items) >= p.config.MaxItems {
			break
		}
	}
	return items, false, nil
}

// hgNormalizeHTTPS 将 Bilibili 返回的明文 CDN 地址升级为 HTTPS；其他 scheme 原样返回供上层决定是否展示。
func hgNormalizeHTTPS(rawURL string) string {
	if strings.HasPrefix(rawURL, "http://") {
		return "https://" + strings.TrimPrefix(rawURL, "http://")
	}
	return rawURL
}
