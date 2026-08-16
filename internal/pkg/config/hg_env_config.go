/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-17 22:19:17
 * @LastEditors: Harley harelysoa@qq.com
 * @LastEditTime: 2026-08-16 16:07:12
 * @FilePath: /MLC_GO/internal/pkg/config/hg_env_config.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package ConfigPackage

import (
	"fmt"
	"net/netip"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

var hgStatisticGenerationPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,32}$`)
var hgAPIVersionPattern = regexp.MustCompile(`^v[1-9][0-9]{0,2}$`)

// HGMySQLConfig 描述当前环境的 MySQL 连接和 schema 版本约束。
type HGMySQLConfig struct {
	// yaml是结构体标签，对应配置文件中的属性
	// mapstructure 是：将 map 数据映射到 struct 的库使用的标签。常见于：Viper、配置中心、环境变量转换
	// 有yaml、mapstructure表示支持2种解析方式。
	Host                 string `yaml:"host" mapstructure:"host"`
	Port                 string `yaml:"port" mapstructure:"port"`
	User                 string `yaml:"user" mapstructure:"user"`
	Password             string `yaml:"password" mapstructure:"password"`
	Database             string `yaml:"database" mapstructure:"database"`
	MigrateExpectVersion int    `yaml:"migrate_expect_version" mapstructure:"migrate_expect_version"`
}

// HGRedisConfig 描述当前环境的 Redis 网络地址。
type HGRedisConfig struct {
	Host string `yaml:"host" mapstructure:"host"`
	Port string `yaml:"port" mapstructure:"port"`
}

// HGAPIGatewayModulePolicy 定义单个业务模块按来源 IP 执行的分布式令牌桶参数。
type HGAPIGatewayModulePolicy struct {
	Capacity        int64
	RefillPerSecond float64
	MaxBodyBytes    int64
	MaxInFlight     int
	UpstreamURL     string
}

// HGAPIGatewayConfig 定义业务 HTTP 入口的版本、资源、限流和可信代理边界。
type HGAPIGatewayConfig struct {
	Enabled           bool
	MaxURLBytes       int
	SupportedVersions map[string]struct{}
	TrustedProxyCIDRs []netip.Prefix
	Modules           map[string]HGAPIGatewayModulePolicy
}

// HGVideoRecommendConfig 定义 Feed 读写双方共享的 Redis 投影版本和固定分片数。
type HGVideoRecommendConfig struct {
	RedisGeneration string
	RedisShardCount int
	RedisMaxItems   int
}

// HGIDGeneratorConfig 描述业务 ID 的固定纪元和当前实例 Worker ID。
type HGIDGeneratorConfig struct {
	Epoch    time.Time
	WorkerID int64
}

type hgIDGeneratorRawConfig struct {
	Epoch    string `yaml:"epoch" mapstructure:"epoch"`
	WorkerID int64  `yaml:"worker_id" mapstructure:"worker_id"`
}

// HGClickHouseConfig 描述 Statistic 权威事件存储的 HTTP 连接配置。
// 它是ClickHouse 数据库配置对象，读取后就可以用于创建 ClickHouse 客户端
type HGClickHouseConfig struct {
	Enabled              bool          `yaml:"enabled" mapstructure:"enabled"`
	Scheme               string        `yaml:"scheme" mapstructure:"scheme"`
	Host                 string        `yaml:"host" mapstructure:"host"`
	Port                 string        `yaml:"port" mapstructure:"port"`
	Database             string        `yaml:"database" mapstructure:"database"`
	User                 string        `yaml:"user" mapstructure:"user"`
	Password             string        `yaml:"password" mapstructure:"password"`
	StatisticEventsTable string        `yaml:"statistic_events_table" mapstructure:"statistic_events_table"`
	StatisticTotalsTable string        `yaml:"statistic_totals_table" mapstructure:"statistic_totals_table"`
	DanmakuHistoryTable  string        `yaml:"danmaku_history_table" mapstructure:"danmaku_history_table"`
	WriteTimeout         string        `yaml:"write_timeout" mapstructure:"write_timeout"`
	QueryTimeout         string        `yaml:"query_timeout" mapstructure:"query_timeout"`
	WriteTimeoutDuration time.Duration `yaml:"-" mapstructure:"-"`
	QueryTimeoutDuration time.Duration `yaml:"-" mapstructure:"-"`
}

// HGStatisticConfig 描述 Redis 投影版本和检测式对账参数。
// 从配置文件中直接读取出来的“原始统计配置”
type HGStatisticConfig struct {
	RedisGeneration   string `yaml:"redis_generation" mapstructure:"redis_generation"`
	RedisShardCount   int    `yaml:"redis_shard_count" mapstructure:"redis_shard_count"`
	ReconcileEnabled  bool   `yaml:"reconcile_enabled" mapstructure:"reconcile_enabled"`
	ReconcileInterval string `yaml:"reconcile_interval" mapstructure:"reconcile_interval"`
	ReconcileTimeout  string `yaml:"reconcile_timeout" mapstructure:"reconcile_timeout"`
}

// HGStatisticInfrastructureConfig 是校验完成、可直接构造运行期依赖的配置。
// 它是经过整理、转换之后，真正给业务基础设施使用的配置
type HGStatisticInfrastructureConfig struct {
	RedisGeneration   string
	RedisShardCount   int
	ReconcileEnabled  bool
	ReconcileInterval time.Duration
	ReconcileTimeout  time.Duration
}

// HGInteractionReprojectConfig bounds the periodic MySQL-to-Redis interaction repair worker.
type HGInteractionReprojectConfig struct {
	Enabled     bool
	Interval    time.Duration
	Timeout     time.Duration
	SafetyLag   time.Duration
	LeaseTTL    time.Duration
	PageSize    int
	WorkerCount int
	HashRanges  []HGInteractionReprojectHashRange
}

// HGInteractionReprojectHashRange is a fixed half-open range in the 1024 stored hash buckets.
type HGInteractionReprojectHashRange struct {
	Start uint16 `mapstructure:"start"`
	End   uint16 `mapstructure:"end"`
}

// HGCoinJobConfig bounds initialization, expiration, and reconciliation database work.
type HGCoinJobConfig struct {
	Enabled                   bool
	Interval                  time.Duration
	Timeout                   time.Duration
	BatchSize                 int
	ConsolidationBatchSize    int
	ConsolidationSourceLimit  int
	ConsolidationMaxLotAmount uint64
}

// HGCorrectionRecoveryConfig bounds stale approving scans and idempotent retries.
type HGCorrectionRecoveryConfig struct {
	Enabled          bool
	Interval         time.Duration
	Timeout          time.Duration
	ApprovingTimeout time.Duration
	BatchSize        int
}

// HGCrawlerConfig 控制主应用是否内嵌启动 Bilibili 周期 worker。
// 独立 cmd/hg_crawler 可以不依赖该开关运行；主应用默认关闭，避免多副本部署放大第三方请求。
type HGCrawlerConfig struct {
	Enabled       bool
	Interval      time.Duration
	Timeout       time.Duration
	MaxItems      int
	RetryCount    int
	RatePerSecond float64
	UserAgent     string
}

// HGVideoCommentStorageConfig 描述 local/S3 存储和 CDN；S3 凭据只从环境变量注入。
type HGVideoCommentStorageConfig struct {
	Type, Endpoint, Region, Bucket, CDNBaseURL, AccessKeyID, SecretAccessKey string
	RequestTimeout                                                           time.Duration
}

// HGVideoCommentImageConfig 定义用户权威容量（字节）和用户/IP 令牌桶参数。
type HGVideoCommentImageConfig struct {
	UserCapacityBytes int64
	RateUserCapacity  int64
	RateIPCapacity    int64
	RateWindow        time.Duration
}

// HGVideoCommentMaintenanceConfig 定义赞踩/回复计数投影与孤儿图片清理的有界后台任务参数。
type HGVideoCommentMaintenanceConfig struct {
	Enabled                      bool
	Interval, Timeout, OrphanAge time.Duration
	BatchSize                    int
}

// HGVideoCommentConfig 汇总评论图片存储、限流配额和后台维护配置。
type HGVideoCommentConfig struct {
	Storage     HGVideoCommentStorageConfig
	Image       HGVideoCommentImageConfig
	Maintenance HGVideoCommentMaintenanceConfig
	// TrustedProxyCIDRs 仅用于决定是否信任 X-Forwarded-For/X-Real-IP，不应配置客户端可直连的宽泛网段。
	TrustedProxyCIDRs []netip.Prefix
}

// HGVideoDanmakuConfig 定义独立 gnet 网关、分片房间、队列、限流、帧和票据的硬资源边界。
type HGVideoDanmakuConfig struct {
	Host                                                                     string
	Port                                                                     string
	AllowedOrigins                                                           []string
	TicketTTL, HeartbeatInterval, HeartbeatTimeout, DrainTimeout             time.Duration
	WorkerCount, QueueSize, MaxConnections, MaxFrameBytes, MaxHandshakeBytes int
	RoomShardCount, MemberShardCount, HeartbeatShardCount, MaxPendingBytes   int
	CommandRatePerSecond, CommandBurst                                       int
	BroadcastWorkerCount, BroadcastQueueSize, RecentMessageLimit             int
}

// GetVideoDanmakuConfig 读取并校验弹幕实时网关配置。
func GetVideoDanmakuConfig() (HGVideoDanmakuConfig, error) {
	var raw struct {
		Host              string   `mapstructure:"host"`
		Port              string   `mapstructure:"port"`
		AllowedOrigins    []string `mapstructure:"allowed_origins"`
		TicketTTL         string   `mapstructure:"ticket_ttl"`
		HeartbeatInterval string   `mapstructure:"heartbeat_interval"`
		HeartbeatTimeout  string   `mapstructure:"heartbeat_timeout"`
		DrainTimeout      string   `mapstructure:"drain_timeout"`
		WorkerCount       int      `mapstructure:"worker_count"`
		QueueSize         int      `mapstructure:"queue_size"`
		MaxConnections    int      `mapstructure:"max_connections"`
		MaxFrameBytes     int      `mapstructure:"max_frame_bytes"`
		MaxHandshakeBytes int      `mapstructure:"max_handshake_bytes"`
		RoomShardCount    int      `mapstructure:"room_shard_count"`
		MemberShardCount  int      `mapstructure:"member_shard_count"`
		HeartbeatShards   int      `mapstructure:"heartbeat_shard_count"`
		MaxPendingBytes   int      `mapstructure:"max_pending_bytes"`
		CommandRate       int      `mapstructure:"command_rate_per_second"`
		CommandBurst      int      `mapstructure:"command_burst"`
		BroadcastWorkers  int      `mapstructure:"broadcast_worker_count"`
		BroadcastQueue    int      `mapstructure:"broadcast_queue_size"`
		RecentLimit       int      `mapstructure:"recent_message_limit"`
	}
	var cfg HGVideoDanmakuConfig
	if err := viper.UnmarshalKey("video_danmaku", &raw); err != nil {
		return cfg, fmt.Errorf("读取 video danmaku 配置失败: %w", err)
	}
	cfg.Host, cfg.Port = strings.TrimSpace(raw.Host), strings.TrimSpace(raw.Port)
	if cfg.Host == "" {
		cfg.Host = "0.0.0.0"
	}
	if err := hgValidatePort("video_danmaku.port", cfg.Port); err != nil {
		return cfg, err
	}
	var err error
	if cfg.TicketTTL, err = time.ParseDuration(raw.TicketTTL); err != nil || cfg.TicketTTL < 10*time.Second || cfg.TicketTTL > 5*time.Minute {
		return cfg, fmt.Errorf("video_danmaku.ticket_ttl 必须在 10s-5m 之间")
	}
	if cfg.HeartbeatInterval, err = time.ParseDuration(raw.HeartbeatInterval); err != nil || cfg.HeartbeatInterval < 5*time.Second || cfg.HeartbeatInterval > time.Minute {
		return cfg, fmt.Errorf("video_danmaku.heartbeat_interval 必须在 5s-1m 之间")
	}
	if cfg.HeartbeatTimeout, err = time.ParseDuration(raw.HeartbeatTimeout); err != nil || cfg.HeartbeatTimeout < 2*cfg.HeartbeatInterval || cfg.HeartbeatTimeout > 5*time.Minute {
		return cfg, fmt.Errorf("video_danmaku.heartbeat_timeout 必须在 heartbeat_interval 的 2 倍到 5m 之间")
	}
	if strings.TrimSpace(raw.DrainTimeout) == "" {
		raw.DrainTimeout = "30s"
	}
	if cfg.DrainTimeout, err = time.ParseDuration(raw.DrainTimeout); err != nil || cfg.DrainTimeout < 5*time.Second || cfg.DrainTimeout > 30*time.Second {
		return cfg, fmt.Errorf("video_danmaku.drain_timeout 必须在 5s-30s 之间")
	}
	cfg.WorkerCount, cfg.QueueSize, cfg.MaxConnections, cfg.MaxFrameBytes, cfg.MaxHandshakeBytes = raw.WorkerCount, raw.QueueSize, raw.MaxConnections, raw.MaxFrameBytes, raw.MaxHandshakeBytes
	cfg.RoomShardCount, cfg.MemberShardCount, cfg.HeartbeatShardCount = raw.RoomShardCount, raw.MemberShardCount, raw.HeartbeatShards
	cfg.MaxPendingBytes = raw.MaxPendingBytes
	cfg.CommandRatePerSecond, cfg.CommandBurst = raw.CommandRate, raw.CommandBurst
	cfg.BroadcastWorkerCount, cfg.BroadcastQueueSize, cfg.RecentMessageLimit = raw.BroadcastWorkers, raw.BroadcastQueue, raw.RecentLimit
	if cfg.WorkerCount < 1 || cfg.WorkerCount > 256 || cfg.QueueSize < 100 || cfg.QueueSize > 1_000_000 || cfg.MaxConnections < 1 || cfg.MaxFrameBytes < 256 || cfg.MaxFrameBytes > 16<<10 || cfg.MaxHandshakeBytes < 1024 || cfg.MaxHandshakeBytes > 32<<10 || !hgPowerOfTwo(cfg.RoomShardCount, 16, 4096) || !hgPowerOfTwo(cfg.MemberShardCount, 4, 256) || !hgPowerOfTwo(cfg.HeartbeatShardCount, 4, 256) || cfg.MaxPendingBytes < 16<<10 || cfg.MaxPendingBytes > 1<<20 || cfg.CommandRatePerSecond < 1 || cfg.CommandRatePerSecond > 100 || cfg.CommandBurst < cfg.CommandRatePerSecond || cfg.CommandBurst > 500 || !hgPowerOfTwo(cfg.BroadcastWorkerCount, 1, 256) || cfg.BroadcastQueueSize < 64 || cfg.BroadcastQueueSize > 1_000_000 || cfg.RecentMessageLimit < 100 || cfg.RecentMessageLimit > 10_000 {
		return cfg, fmt.Errorf("video_danmaku 资源边界配置无效")
	}
	allowedOrigins := raw.AllowedOrigins
	// 生产域名通常由发布环境决定，允许用逗号分隔环境变量覆盖 YAML，避免镜像内固化站点域名。
	if value := strings.TrimSpace(os.Getenv("VIDEO_DANMAKU_ALLOWED_ORIGINS")); value != "" {
		allowedOrigins = strings.Split(value, ",")
	}
	for _, origin := range allowedOrigins {
		origin = strings.TrimRight(strings.TrimSpace(origin), "/")
		if origin != "" {
			cfg.AllowedOrigins = append(cfg.AllowedOrigins, origin)
		}
	}
	if len(cfg.AllowedOrigins) == 0 {
		return cfg, fmt.Errorf("video_danmaku.allowed_origins 不能为空")
	}
	return cfg, nil
}

func hgPowerOfTwo(value, minValue, maxValue int) bool {
	return value >= minValue && value <= maxValue && value&(value-1) == 0
}

// GetVideoCommentConfig validates storage, abuse controls and bounded maintenance work.
func GetVideoCommentConfig() (HGVideoCommentConfig, error) {
	var raw struct {
		TrustedProxyCIDRs []string `mapstructure:"trusted_proxy_cidrs"`
		Storage           struct {
			Type           string `mapstructure:"type"`
			Endpoint       string `mapstructure:"endpoint"`
			Region         string `mapstructure:"region"`
			Bucket         string `mapstructure:"bucket"`
			CDNBaseURL     string `mapstructure:"cdn_base_url"`
			RequestTimeout string `mapstructure:"request_timeout"`
		} `mapstructure:"storage"`
		Image struct {
			UserCapacityBytes int64  `mapstructure:"user_capacity_bytes"`
			RateUserCapacity  int64  `mapstructure:"rate_user_capacity"`
			RateIPCapacity    int64  `mapstructure:"rate_ip_capacity"`
			RateWindow        string `mapstructure:"rate_window"`
		} `mapstructure:"image"`
		Maintenance struct {
			Enabled   bool   `mapstructure:"enabled"`
			Interval  string `mapstructure:"interval"`
			Timeout   string `mapstructure:"timeout"`
			OrphanAge string `mapstructure:"orphan_age"`
			BatchSize int    `mapstructure:"batch_size"`
		} `mapstructure:"maintenance"`
	}
	var cfg HGVideoCommentConfig
	if err := viper.UnmarshalKey("video_comment", &raw); err != nil {
		return cfg, fmt.Errorf("读取 video comment 配置失败: %w", err)
	}
	if err := viper.UnmarshalKey("video_comment.storage", &raw.Storage); err != nil {
		return cfg, err
	}
	if err := viper.UnmarshalKey("video_comment.image", &raw.Image); err != nil {
		return cfg, err
	}
	if err := viper.UnmarshalKey("video_comment.maintenance", &raw.Maintenance); err != nil {
		return cfg, err
	}
	trustedProxyCIDRs := raw.TrustedProxyCIDRs
	// 发布系统可通过逗号分隔环境变量覆盖 YAML，避免把基础设施网段固化进镜像配置。
	if value := strings.TrimSpace(os.Getenv("VIDEO_COMMENT_TRUSTED_PROXY_CIDRS")); value != "" {
		trustedProxyCIDRs = strings.Split(value, ",")
	}
	for _, value := range trustedProxyCIDRs {
		prefix, parseErr := netip.ParsePrefix(strings.TrimSpace(value))
		if parseErr != nil {
			return cfg, fmt.Errorf("video_comment.trusted_proxy_cidrs 包含无效 CIDR")
		}
		if prefix.Bits() == 0 {
			return cfg, fmt.Errorf("video_comment.trusted_proxy_cidrs 禁止信任全部地址")
		}
		// Masked 统一主机位，确保后续 Contains 比较和配置输出使用规范网络地址。
		cfg.TrustedProxyCIDRs = append(cfg.TrustedProxyCIDRs, prefix.Masked())
	}
	cfg.Storage.Type = strings.ToLower(strings.TrimSpace(raw.Storage.Type))
	cfg.Storage.Endpoint = strings.TrimRight(strings.TrimSpace(raw.Storage.Endpoint), "/")
	cfg.Storage.Region = strings.TrimSpace(raw.Storage.Region)
	cfg.Storage.Bucket = strings.TrimSpace(raw.Storage.Bucket)
	cfg.Storage.CDNBaseURL = strings.TrimRight(strings.TrimSpace(raw.Storage.CDNBaseURL), "/")
	cfg.Storage.AccessKeyID = os.Getenv("VIDEO_COMMENT_S3_ACCESS_KEY_ID")
	cfg.Storage.SecretAccessKey = os.Getenv("VIDEO_COMMENT_S3_SECRET_ACCESS_KEY")
	if value := strings.TrimSpace(os.Getenv("VIDEO_COMMENT_S3_ENDPOINT")); value != "" {
		cfg.Storage.Endpoint = strings.TrimRight(value, "/")
	}
	if value := strings.TrimSpace(os.Getenv("VIDEO_COMMENT_S3_REGION")); value != "" {
		cfg.Storage.Region = value
	}
	if value := strings.TrimSpace(os.Getenv("VIDEO_COMMENT_S3_BUCKET")); value != "" {
		cfg.Storage.Bucket = value
	}
	if value := strings.TrimSpace(os.Getenv("VIDEO_COMMENT_CDN_BASE_URL")); value != "" {
		cfg.Storage.CDNBaseURL = strings.TrimRight(value, "/")
	}
	var err error
	if cfg.Storage.RequestTimeout, err = time.ParseDuration(raw.Storage.RequestTimeout); err != nil || cfg.Storage.RequestTimeout <= 0 {
		return cfg, fmt.Errorf("video_comment.storage.request_timeout 必须是正 duration")
	}
	if cfg.Storage.Type != "local" && cfg.Storage.Type != "s3" {
		return cfg, fmt.Errorf("video_comment.storage.type 仅支持 local 或 s3")
	}
	if cfg.Storage.Type == "s3" && (cfg.Storage.Endpoint == "" || cfg.Storage.Region == "" || cfg.Storage.Bucket == "" || cfg.Storage.CDNBaseURL == "" || cfg.Storage.AccessKeyID == "" || cfg.Storage.SecretAccessKey == "") {
		return cfg, fmt.Errorf("video_comment S3/CDN 配置或环境凭据不完整")
	}
	cfg.Image.UserCapacityBytes = raw.Image.UserCapacityBytes
	cfg.Image.RateUserCapacity = raw.Image.RateUserCapacity
	cfg.Image.RateIPCapacity = raw.Image.RateIPCapacity
	if cfg.Image.UserCapacityBytes < 5<<20 || cfg.Image.RateUserCapacity < 1 || cfg.Image.RateIPCapacity < 1 {
		return cfg, fmt.Errorf("video_comment.image 容量和限流配置无效")
	}
	if cfg.Image.RateWindow, err = time.ParseDuration(raw.Image.RateWindow); err != nil || cfg.Image.RateWindow <= 0 {
		return cfg, fmt.Errorf("video_comment.image.rate_window 必须是正 duration")
	}
	cfg.Maintenance.Enabled = raw.Maintenance.Enabled
	if cfg.Maintenance.Enabled {
		if cfg.Maintenance.Interval, err = time.ParseDuration(raw.Maintenance.Interval); err != nil || cfg.Maintenance.Interval <= 0 {
			return cfg, fmt.Errorf("video_comment.maintenance.interval 无效")
		}
		if cfg.Maintenance.Timeout, err = time.ParseDuration(raw.Maintenance.Timeout); err != nil || cfg.Maintenance.Timeout <= 0 || cfg.Maintenance.Timeout >= cfg.Maintenance.Interval {
			return cfg, fmt.Errorf("video_comment.maintenance.timeout 必须小于 interval")
		}
		if cfg.Maintenance.OrphanAge, err = time.ParseDuration(raw.Maintenance.OrphanAge); err != nil || cfg.Maintenance.OrphanAge <= cfg.Maintenance.Timeout {
			return cfg, fmt.Errorf("video_comment.maintenance.orphan_age 无效")
		}
		cfg.Maintenance.BatchSize = raw.Maintenance.BatchSize
		if cfg.Maintenance.BatchSize < 1 || cfg.Maintenance.BatchSize > 1000 {
			return cfg, fmt.Errorf("video_comment.maintenance.batch_size 必须在 1-1000 之间")
		}
	}
	return cfg, nil
}

// GetCorrectionRecoveryConfig validates the standalone correction recovery worker limits.
func GetCorrectionRecoveryConfig() (HGCorrectionRecoveryConfig, error) {
	var raw struct {
		Enabled          bool   `mapstructure:"enabled"`
		Interval         string `mapstructure:"interval"`
		Timeout          string `mapstructure:"timeout"`
		ApprovingTimeout string `mapstructure:"approving_timeout"`
		BatchSize        int    `mapstructure:"batch_size"`
	}
	var cfg HGCorrectionRecoveryConfig
	if err := viper.UnmarshalKey("correction_recovery", &raw); err != nil {
		return cfg, fmt.Errorf("读取 correction recovery 配置失败: %w", err)
	}
	cfg.Enabled = raw.Enabled
	if !cfg.Enabled {
		return cfg, nil
	}
	var err error
	if cfg.Interval, err = time.ParseDuration(raw.Interval); err != nil || cfg.Interval <= 0 {
		return cfg, fmt.Errorf("correction_recovery.interval 必须是正 duration")
	}
	if cfg.Timeout, err = time.ParseDuration(raw.Timeout); err != nil || cfg.Timeout <= 0 || cfg.Timeout >= cfg.Interval {
		return cfg, fmt.Errorf("correction_recovery.timeout 必须为正且小于 interval")
	}
	if cfg.ApprovingTimeout, err = time.ParseDuration(raw.ApprovingTimeout); err != nil || cfg.ApprovingTimeout <= cfg.Timeout {
		return cfg, fmt.Errorf("correction_recovery.approving_timeout 必须大于 timeout")
	}
	cfg.BatchSize = raw.BatchSize
	if cfg.BatchSize < 1 || cfg.BatchSize > 100 {
		return cfg, fmt.Errorf("correction_recovery.batch_size 必须在 1-100 之间")
	}
	return cfg, nil
}

// GetCrawlerConfig 读取并校验主应用内嵌 crawler 的有界运行参数。
// CRAWLER_BILIBILI_ENABLED 可覆盖 YAML 开关，便于同一镜像只在指定单副本实例启用 worker。
func GetCrawlerConfig() (HGCrawlerConfig, error) {
	var raw struct {
		Enabled       bool    `mapstructure:"enabled"`
		Interval      string  `mapstructure:"interval"`
		Timeout       string  `mapstructure:"timeout"`
		MaxItems      int     `mapstructure:"max_items"`
		RetryCount    int     `mapstructure:"retry_count"`
		RatePerSecond float64 `mapstructure:"rate_per_second"`
		UserAgent     string  `mapstructure:"user_agent"`
	}
	var cfg HGCrawlerConfig
	if err := viper.UnmarshalKey("crawler.bilibili", &raw); err != nil {
		return cfg, fmt.Errorf("读取 crawler.bilibili 配置失败: %w", err)
	}
	if value := strings.TrimSpace(os.Getenv("CRAWLER_BILIBILI_ENABLED")); value != "" {
		enabled, err := strconv.ParseBool(value)
		if err != nil {
			return cfg, fmt.Errorf("CRAWLER_BILIBILI_ENABLED 必须是布尔值: %w", err)
		}
		raw.Enabled = enabled
	}
	cfg.Enabled = raw.Enabled
	if !cfg.Enabled {
		return cfg, nil
	}
	var err error
	if cfg.Interval, err = time.ParseDuration(raw.Interval); err != nil || cfg.Interval < 10*time.Second {
		return cfg, fmt.Errorf("crawler.bilibili.interval 必须不小于 10s")
	}
	if cfg.Timeout, err = time.ParseDuration(raw.Timeout); err != nil || cfg.Timeout <= 0 || cfg.Timeout >= cfg.Interval || cfg.Timeout > time.Minute {
		return cfg, fmt.Errorf("crawler.bilibili.timeout 必须为正、小于 interval 且不超过 1m")
	}
	cfg.MaxItems = raw.MaxItems
	if cfg.MaxItems < 1 || cfg.MaxItems > 50 {
		return cfg, fmt.Errorf("crawler.bilibili.max_items 必须在 1-50 之间")
	}
	cfg.RetryCount = raw.RetryCount
	if cfg.RetryCount < 0 || cfg.RetryCount > 3 {
		return cfg, fmt.Errorf("crawler.bilibili.retry_count 必须在 0-3 之间")
	}
	cfg.RatePerSecond = raw.RatePerSecond
	if cfg.RatePerSecond <= 0 || cfg.RatePerSecond > 1 {
		return cfg, fmt.Errorf("crawler.bilibili.rate_per_second 必须在 0-1 之间")
	}
	cfg.UserAgent = strings.TrimSpace(raw.UserAgent)
	if cfg.UserAgent == "" || len(cfg.UserAgent) > 128 {
		return cfg, fmt.Errorf("crawler.bilibili.user_agent 不能为空且不能超过 128 字节")
	}
	return cfg, nil
}

// GetCoinJobConfig validates the fixed upper bounds used by coin background work.
func GetCoinJobConfig() (HGCoinJobConfig, error) {
	var raw struct {
		Enabled                   bool   `mapstructure:"enabled"`
		Interval                  string `mapstructure:"interval"`
		Timeout                   string `mapstructure:"timeout"`
		BatchSize                 int    `mapstructure:"batch_size"`
		ConsolidationBatchSize    int    `mapstructure:"consolidation_batch_size"`
		ConsolidationSourceLimit  int    `mapstructure:"consolidation_source_limit"`
		ConsolidationMaxLotAmount uint64 `mapstructure:"consolidation_max_lot_amount"`
	}
	var cfg HGCoinJobConfig
	if err := viper.UnmarshalKey("coin_jobs", &raw); err != nil {
		return cfg, fmt.Errorf("读取 Coin jobs 配置失败: %w", err)
	}
	cfg.Enabled = raw.Enabled
	if !cfg.Enabled {
		return cfg, nil
	}
	var err error
	if cfg.Interval, err = time.ParseDuration(raw.Interval); err != nil || cfg.Interval <= 0 {
		return cfg, fmt.Errorf("coin_jobs.interval 必须是正 duration")
	}
	if cfg.Timeout, err = time.ParseDuration(raw.Timeout); err != nil || cfg.Timeout <= 0 || cfg.Timeout >= cfg.Interval {
		return cfg, fmt.Errorf("coin_jobs.timeout 必须为正且小于 interval")
	}
	cfg.BatchSize = raw.BatchSize
	if cfg.BatchSize < 1 || cfg.BatchSize > 1000 {
		return cfg, fmt.Errorf("coin_jobs.batch_size 必须在 1-1000 之间")
	}
	cfg.ConsolidationBatchSize = raw.ConsolidationBatchSize
	cfg.ConsolidationSourceLimit = raw.ConsolidationSourceLimit
	cfg.ConsolidationMaxLotAmount = raw.ConsolidationMaxLotAmount
	if cfg.ConsolidationBatchSize < 0 || cfg.ConsolidationBatchSize > 1000 {
		return cfg, fmt.Errorf("coin_jobs.consolidation_batch_size 必须在 0-1000 之间")
	}
	if cfg.ConsolidationBatchSize > 0 && (cfg.ConsolidationSourceLimit < 2 || cfg.ConsolidationSourceLimit > 1000 || cfg.ConsolidationMaxLotAmount == 0) {
		return cfg, fmt.Errorf("coin_jobs consolidation 配置无效")
	}
	return cfg, nil
}

// GetInteractionReprojectConfig reads and validates the standalone interaction reprojector limits.
func GetInteractionReprojectConfig() (HGInteractionReprojectConfig, error) {
	var raw struct {
		Enabled     bool                              `mapstructure:"enabled"`
		Interval    string                            `mapstructure:"interval"`
		Timeout     string                            `mapstructure:"timeout"`
		SafetyLag   string                            `mapstructure:"safety_lag"`
		LeaseTTL    string                            `mapstructure:"lease_ttl"`
		PageSize    int                               `mapstructure:"page_size"`
		WorkerCount int                               `mapstructure:"worker_count"`
		HashRanges  []HGInteractionReprojectHashRange `mapstructure:"hash_ranges"`
	}
	var cfg HGInteractionReprojectConfig
	if err := viper.UnmarshalKey("interaction_reproject", &raw); err != nil {
		return cfg, fmt.Errorf("读取 Interaction reproject 配置失败: %w", err)
	}
	cfg.Enabled = raw.Enabled
	if !cfg.Enabled {
		return cfg, nil
	}
	var err error
	if cfg.Interval, err = time.ParseDuration(raw.Interval); err != nil || cfg.Interval <= 0 {
		return cfg, fmt.Errorf("interaction_reproject.interval 必须是正 duration")
	}
	if cfg.Timeout, err = time.ParseDuration(raw.Timeout); err != nil || cfg.Timeout <= 0 || cfg.Timeout >= cfg.Interval {
		return cfg, fmt.Errorf("interaction_reproject.timeout 必须为正且小于 interval")
	}
	if cfg.SafetyLag, err = time.ParseDuration(raw.SafetyLag); err != nil || cfg.SafetyLag <= 0 {
		return cfg, fmt.Errorf("interaction_reproject.safety_lag 必须是正 duration")
	}
	if cfg.LeaseTTL, err = time.ParseDuration(raw.LeaseTTL); err != nil || cfg.LeaseTTL <= cfg.Timeout {
		return cfg, fmt.Errorf("interaction_reproject.lease_ttl 必须大于 timeout")
	}
	cfg.PageSize = raw.PageSize
	if cfg.PageSize < 1 || cfg.PageSize > 1000 {
		return cfg, fmt.Errorf("interaction_reproject.page_size 必须在 1-1000 之间")
	}
	cfg.WorkerCount = raw.WorkerCount
	if cfg.WorkerCount == 0 {
		cfg.WorkerCount = 1
	}
	if cfg.WorkerCount < 1 || cfg.WorkerCount > 32 {
		return cfg, fmt.Errorf("interaction_reproject.worker_count 必须在 1-32 之间")
	}
	cfg.HashRanges = raw.HashRanges
	if len(cfg.HashRanges) == 0 {
		cfg.HashRanges = []HGInteractionReprojectHashRange{{Start: 0, End: 1024}}
	}
	for i, hashRange := range cfg.HashRanges {
		if hashRange.Start >= hashRange.End || hashRange.End > 1024 || (i > 0 && cfg.HashRanges[i-1].End > hashRange.Start) {
			return cfg, fmt.Errorf("interaction_reproject.hash_ranges 配置无效")
		}
	}
	return cfg, nil
}

// GetMySQLConfig 从已加载的模块化 YAML 中读取并校验 MySQL 配置。
func GetMySQLConfig() (HGMySQLConfig, error) {
	var cfg HGMySQLConfig
	// viper.UnmarshalKey 读取 viper 配置中 mysql 节点的内容，并将其反序列化到 cfg 结构体中。这个时候结构体中的标签mapstructure起到作用了。
	if err := viper.UnmarshalKey("mysql", &cfg); err != nil {
		return cfg, fmt.Errorf("读取 MySQL 配置失败: %w", err)
	}
	// debug 密码允许由本机未跟踪文件或进程环境覆盖；LookupEnv 可区分“未设置”和“显式空密码”。
	// pre/prod 始终使用各自部署配置或密钥注入，不受开发机本地密码影响。
	if Env(viper.GetString(hgLoadedEnvKey)) == EnvDebug {
		if password, exists := os.LookupEnv(hgDebugMySQLPasswordEnv); exists {
			cfg.Password = password
		}
	}
	cfg.Host = strings.TrimSpace(cfg.Host)
	cfg.Port = strings.TrimSpace(cfg.Port)
	cfg.User = strings.TrimSpace(cfg.User)
	cfg.Database = strings.TrimSpace(cfg.Database)
	if cfg.Host == "" || cfg.User == "" || cfg.Database == "" {
		return cfg, fmt.Errorf("mysql.host、mysql.user、mysql.database 不能为空")
	}
	if err := hgValidatePort("mysql.port", cfg.Port); err != nil {
		return cfg, err
	}
	if cfg.MigrateExpectVersion < 1 {
		return cfg, fmt.Errorf("mysql.migrate_expect_version 必须大于等于 1")
	}
	return cfg, nil
}

// GetRedisConfig 从已加载的模块化 YAML 中读取并校验 Redis 配置。
func GetRedisConfig() (HGRedisConfig, error) {
	var cfg HGRedisConfig
	if err := viper.UnmarshalKey("redis", &cfg); err != nil {
		return cfg, fmt.Errorf("读取 Redis 配置失败: %w", err)
	}
	cfg.Host = strings.TrimSpace(cfg.Host)
	cfg.Port = strings.TrimSpace(cfg.Port)
	if cfg.Host == "" {
		return cfg, fmt.Errorf("redis.host 不能为空")
	}
	if err := hgValidatePort("redis.port", cfg.Port); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// GetAPIGatewayConfig 读取并校验 API Gateway 配置；模块策略在启动期固化，请求期不读取 Viper。
func GetAPIGatewayConfig() (HGAPIGatewayConfig, error) {
	var raw struct {
		Enabled           bool     `mapstructure:"enabled"`
		MaxURLBytes       int      `mapstructure:"max_url_bytes"`
		SupportedVersions []string `mapstructure:"supported_versions"`
		TrustedProxyCIDRs []string `mapstructure:"trusted_proxy_cidrs"`
		Modules           map[string]struct {
			Capacity        int64   `mapstructure:"capacity"`
			RefillPerSecond float64 `mapstructure:"refill_per_second"`
			MaxBodyBytes    int64   `mapstructure:"max_body_bytes"`
			MaxInFlight     int     `mapstructure:"max_in_flight"`
			UpstreamURL     string  `mapstructure:"upstream_url"`
		} `mapstructure:"modules"`
	}
	var cfg HGAPIGatewayConfig
	if err := viper.UnmarshalKey("api_gateway", &raw); err != nil {
		return cfg, fmt.Errorf("读取 API Gateway 配置失败: %w", err)
	}
	cfg.Enabled = raw.Enabled
	if !cfg.Enabled {
		return cfg, nil
	}
	if raw.MaxURLBytes < 1024 || raw.MaxURLBytes > 64<<10 {
		return cfg, fmt.Errorf("api_gateway.max_url_bytes 必须在 1KiB-64KiB 之间")
	}
	cfg.MaxURLBytes = raw.MaxURLBytes
	cfg.SupportedVersions = make(map[string]struct{}, len(raw.SupportedVersions))
	for _, version := range raw.SupportedVersions {
		version = strings.TrimSpace(version)
		if !hgAPIVersionPattern.MatchString(version) {
			return cfg, fmt.Errorf("api_gateway.supported_versions 包含无效版本")
		}
		cfg.SupportedVersions[version] = struct{}{}
	}
	if len(cfg.SupportedVersions) == 0 {
		return cfg, fmt.Errorf("api_gateway.supported_versions 不能为空")
	}

	trustedProxyCIDRs := raw.TrustedProxyCIDRs
	if value := strings.TrimSpace(os.Getenv("API_GATEWAY_TRUSTED_PROXY_CIDRS")); value != "" {
		trustedProxyCIDRs = strings.Split(value, ",")
	}
	for _, value := range trustedProxyCIDRs {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(value))
		if err != nil {
			return cfg, fmt.Errorf("api_gateway.trusted_proxy_cidrs 包含无效 CIDR")
		}
		if prefix.Bits() == 0 {
			return cfg, fmt.Errorf("api_gateway.trusted_proxy_cidrs 禁止信任全部地址")
		}
		cfg.TrustedProxyCIDRs = append(cfg.TrustedProxyCIDRs, prefix.Masked())
	}

	allowedModules := map[string]struct{}{
		"auth": {}, "profile": {}, "video_upload": {}, "bilibili": {},
		"video_recommend": {}, "video_interaction": {}, "video_comment": {}, "video_danmaku": {}, "ops": {},
	}
	if len(raw.Modules) != len(allowedModules) {
		return cfg, fmt.Errorf("api_gateway.modules 必须完整配置 %d 个业务模块", len(allowedModules))
	}
	cfg.Modules = make(map[string]HGAPIGatewayModulePolicy, len(raw.Modules))
	for module, policy := range raw.Modules {
		if _, ok := allowedModules[module]; !ok {
			return cfg, fmt.Errorf("api_gateway.modules 包含未知模块 %q", module)
		}
		if policy.Capacity < 1 || policy.Capacity > 1_000_000 || policy.RefillPerSecond <= 0 || policy.RefillPerSecond > 1_000_000 || policy.MaxBodyBytes < 1 || policy.MaxBodyBytes > 8<<30 || policy.MaxInFlight < 1 || policy.MaxInFlight > 1_000_000 {
			return cfg, fmt.Errorf("api_gateway.modules.%s 限流参数无效", module)
		}
		upstreamURL := strings.TrimRight(strings.TrimSpace(policy.UpstreamURL), "/")
		envName := "API_GATEWAY_UPSTREAM_" + strings.ToUpper(module)
		if value := strings.TrimSpace(os.Getenv(envName)); value != "" {
			upstreamURL = strings.TrimRight(value, "/")
		}
		if upstreamURL != "" {
			parsed, err := url.Parse(upstreamURL)
			if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
				return cfg, fmt.Errorf("api_gateway.modules.%s.upstream_url 必须是无凭据、路径和查询参数的 HTTP(S) 地址", module)
			}
		}
		cfg.Modules[module] = HGAPIGatewayModulePolicy{
			Capacity: policy.Capacity, RefillPerSecond: policy.RefillPerSecond,
			MaxBodyBytes: policy.MaxBodyBytes, MaxInFlight: policy.MaxInFlight, UpstreamURL: upstreamURL,
		}
	}
	return cfg, nil
}

// GetVideoRecommendConfig 读取并校验推荐 Feed 投影配置；读写两侧必须使用同一 generation 和 shard_count。
func GetVideoRecommendConfig() (HGVideoRecommendConfig, error) {
	var raw struct {
		RedisGeneration string `mapstructure:"redis_generation"`
		RedisShardCount int    `mapstructure:"redis_shard_count"`
		RedisMaxItems   int    `mapstructure:"redis_max_items_per_shard"`
	}
	var cfg HGVideoRecommendConfig
	if err := viper.UnmarshalKey("video_recommend", &raw); err != nil {
		return cfg, fmt.Errorf("读取 video recommend 配置失败: %w", err)
	}
	cfg.RedisGeneration = strings.TrimSpace(raw.RedisGeneration)
	cfg.RedisShardCount = raw.RedisShardCount
	cfg.RedisMaxItems = raw.RedisMaxItems
	if !hgStatisticGenerationPattern.MatchString(cfg.RedisGeneration) {
		return cfg, fmt.Errorf("video_recommend.redis_generation 必须为 1-32 位字母、数字、下划线或连字符")
	}
	if cfg.RedisShardCount < 1 || cfg.RedisShardCount > 4096 {
		return cfg, fmt.Errorf("video_recommend.redis_shard_count 必须在 1-4096 之间")
	}
	if cfg.RedisMaxItems < 100 || cfg.RedisMaxItems > 1_000_000 {
		return cfg, fmt.Errorf("video_recommend.redis_max_items_per_shard 必须在 100-1000000 之间")
	}
	return cfg, nil
}

// GetIDGeneratorConfig 读取 Snowflake 配置；生产环境必须通过环境变量为每个实例分配唯一 Worker ID。
func GetIDGeneratorConfig() (HGIDGeneratorConfig, error) {
	var raw hgIDGeneratorRawConfig
	var cfg HGIDGeneratorConfig
	if err := viper.UnmarshalKey("id_generator", &raw); err != nil {
		return cfg, fmt.Errorf("读取 ID generator 配置失败: %w", err)
	}
	if value := strings.TrimSpace(os.Getenv("ID_GENERATOR_WORKER_ID")); value != "" {
		workerID, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return cfg, fmt.Errorf("ID_GENERATOR_WORKER_ID 必须是整数: %w", err)
		}
		raw.WorkerID = workerID
	} else if IsProd() {
		return cfg, fmt.Errorf("生产环境必须设置 ID_GENERATOR_WORKER_ID")
	}
	epoch, err := time.Parse(time.RFC3339, strings.TrimSpace(raw.Epoch))
	if err != nil {
		return cfg, fmt.Errorf("id_generator.epoch 必须是 RFC3339 时间: %w", err)
	}
	if raw.WorkerID < 0 || raw.WorkerID > 1023 {
		return cfg, fmt.Errorf("id_generator.worker_id 必须在 0-1023 之间")
	}
	cfg.Epoch, cfg.WorkerID = epoch, raw.WorkerID
	return cfg, nil
}

// GetStatisticInfrastructureConfig 读取并校验 ClickHouse 和 Statistic 投影配置。
func GetStatisticInfrastructureConfig() (HGClickHouseConfig, HGStatisticInfrastructureConfig, error) {
	// clickHouse ClickHouse 数据库配置
	var clickHouse HGClickHouseConfig

	// rawStatistic 配置文件原始结构
	var rawStatistic HGStatisticConfig

	// statistic 程序运行时统计基础设施配置
	var statistic HGStatisticInfrastructureConfig
	if err := viper.UnmarshalKey("clickhouse", &clickHouse); err != nil {
		return clickHouse, statistic, fmt.Errorf("读取 ClickHouse 配置失败: %w", err)
	}
	if err := viper.UnmarshalKey("statistic", &rawStatistic); err != nil {
		return clickHouse, statistic, fmt.Errorf("读取 Statistic 配置失败: %w", err)
	}

	// TrimSpace 去掉字符串首尾的空白字符。
	clickHouse.Scheme = strings.TrimSpace(clickHouse.Scheme)
	clickHouse.Host = strings.TrimSpace(clickHouse.Host)
	clickHouse.Port = strings.TrimSpace(clickHouse.Port)
	clickHouse.Database = strings.TrimSpace(clickHouse.Database)
	clickHouse.User = strings.TrimSpace(clickHouse.User)
	clickHouse.StatisticEventsTable = strings.TrimSpace(clickHouse.StatisticEventsTable)
	clickHouse.StatisticTotalsTable = strings.TrimSpace(clickHouse.StatisticTotalsTable)
	clickHouse.DanmakuHistoryTable = strings.TrimSpace(clickHouse.DanmakuHistoryTable)
	if password := os.Getenv("CLICKHOUSE_PASSWORD"); password != "" {
		clickHouse.Password = password
	}
	if clickHouse.Scheme != "http" && clickHouse.Scheme != "https" {
		return clickHouse, statistic, fmt.Errorf("clickhouse.scheme 仅支持 http 或 https")
	}
	if clickHouse.Host == "" || clickHouse.Database == "" || clickHouse.User == "" || clickHouse.StatisticEventsTable == "" || clickHouse.StatisticTotalsTable == "" {
		return clickHouse, statistic, fmt.Errorf("clickhouse.host、database、user 和统计表名不能为空")
	}
	if err := hgValidatePort("clickhouse.port", clickHouse.Port); err != nil {
		return clickHouse, statistic, err
	}

	// ParseDuration 把字符串格式的时间时长，解析成 time.Duration 类型（int64 纳秒），用于表示一段时间长度，不是解析时间点（不是解析年月日时分秒的时刻）。
	writeTimeout, err := time.ParseDuration(clickHouse.WriteTimeout)
	if err != nil || writeTimeout <= 0 {
		return clickHouse, statistic, fmt.Errorf("clickhouse.write_timeout 必须是正 duration")
	}
	queryTimeout, err := time.ParseDuration(clickHouse.QueryTimeout)
	if err != nil || queryTimeout <= 0 {
		return clickHouse, statistic, fmt.Errorf("clickhouse.query_timeout 必须是正 duration")
	}
	clickHouse.WriteTimeoutDuration = writeTimeout
	clickHouse.QueryTimeoutDuration = queryTimeout

	statistic.RedisGeneration = strings.TrimSpace(rawStatistic.RedisGeneration)
	statistic.RedisShardCount = rawStatistic.RedisShardCount
	statistic.ReconcileEnabled = rawStatistic.ReconcileEnabled
	if !hgStatisticGenerationPattern.MatchString(statistic.RedisGeneration) {
		return clickHouse, statistic, fmt.Errorf("statistic.redis_generation 仅允许 1-32 位字母、数字、下划线和连字符")
	}
	if statistic.RedisShardCount < 1 || statistic.RedisShardCount > 4096 {
		return clickHouse, statistic, fmt.Errorf("statistic.redis_shard_count 必须在 1-4096 之间")
	}
	statistic.ReconcileInterval, err = time.ParseDuration(rawStatistic.ReconcileInterval)
	if err != nil || statistic.ReconcileInterval <= 0 {
		return clickHouse, statistic, fmt.Errorf("statistic.reconcile_interval 必须是正 duration")
	}
	statistic.ReconcileTimeout, err = time.ParseDuration(rawStatistic.ReconcileTimeout)
	if err != nil || statistic.ReconcileTimeout <= 0 || statistic.ReconcileTimeout >= statistic.ReconcileInterval {
		return clickHouse, statistic, fmt.Errorf("statistic.reconcile_timeout 必须为正且小于 reconcile_interval")
	}
	return clickHouse, statistic, nil
}

func hgValidatePort(name string, value string) error {
	// Atoi表示十进制字符串→int
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("%s 必须是 1-65535 的有效端口", name)
	}
	return nil
}
