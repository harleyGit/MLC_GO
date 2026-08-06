/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-17 22:19:17
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-07-25 15:24:14
 * @FilePath: /MLC_GO/internal/pkg/config/hg_env_config.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package ConfigPackage

import (
	"fmt"
	"net/netip"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

var hgStatisticGenerationPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,32}$`)

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

// HGClickHouseConfig 描述 Statistic 权威事件存储的 HTTP 连接配置。
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
type HGStatisticConfig struct {
	RedisGeneration   string `yaml:"redis_generation" mapstructure:"redis_generation"`
	RedisShardCount   int    `yaml:"redis_shard_count" mapstructure:"redis_shard_count"`
	ReconcileEnabled  bool   `yaml:"reconcile_enabled" mapstructure:"reconcile_enabled"`
	ReconcileInterval string `yaml:"reconcile_interval" mapstructure:"reconcile_interval"`
	ReconcileTimeout  string `yaml:"reconcile_timeout" mapstructure:"reconcile_timeout"`
}

// HGStatisticInfrastructureConfig 是校验完成、可直接构造运行期依赖的配置。
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

// HGVideoCommentMaintenanceConfig 定义赞踩投影与孤儿图片清理的有界后台任务参数。
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
	Host                                                                                  string
	Port                                                                                  string
	AllowedOrigins                                                                        []string
	TicketTTL                                                                             time.Duration
	WorkerCount, QueueSize, MaxConnections, MaxFrameBytes, MaxHandshakeBytes              int
	RoomShardCount, MemberShardCount, MaxPendingBytes, CommandRatePerSecond, CommandBurst int
	BroadcastWorkerCount, BroadcastQueueSize, RecentMessageLimit                          int
}

// GetVideoDanmakuConfig 读取并校验弹幕实时网关配置。
func GetVideoDanmakuConfig() (HGVideoDanmakuConfig, error) {
	var raw struct {
		Host              string   `mapstructure:"host"`
		Port              string   `mapstructure:"port"`
		AllowedOrigins    []string `mapstructure:"allowed_origins"`
		TicketTTL         string   `mapstructure:"ticket_ttl"`
		WorkerCount       int      `mapstructure:"worker_count"`
		QueueSize         int      `mapstructure:"queue_size"`
		MaxConnections    int      `mapstructure:"max_connections"`
		MaxFrameBytes     int      `mapstructure:"max_frame_bytes"`
		MaxHandshakeBytes int      `mapstructure:"max_handshake_bytes"`
		RoomShardCount    int      `mapstructure:"room_shard_count"`
		MemberShardCount  int      `mapstructure:"member_shard_count"`
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
	cfg.WorkerCount, cfg.QueueSize, cfg.MaxConnections, cfg.MaxFrameBytes, cfg.MaxHandshakeBytes = raw.WorkerCount, raw.QueueSize, raw.MaxConnections, raw.MaxFrameBytes, raw.MaxHandshakeBytes
	cfg.RoomShardCount, cfg.MemberShardCount, cfg.MaxPendingBytes = raw.RoomShardCount, raw.MemberShardCount, raw.MaxPendingBytes
	cfg.CommandRatePerSecond, cfg.CommandBurst = raw.CommandRate, raw.CommandBurst
	cfg.BroadcastWorkerCount, cfg.BroadcastQueueSize, cfg.RecentMessageLimit = raw.BroadcastWorkers, raw.BroadcastQueue, raw.RecentLimit
	if cfg.WorkerCount < 1 || cfg.WorkerCount > 256 || cfg.QueueSize < 100 || cfg.QueueSize > 1_000_000 || cfg.MaxConnections < 1 || cfg.MaxFrameBytes < 256 || cfg.MaxFrameBytes > 16<<10 || cfg.MaxHandshakeBytes < 1024 || cfg.MaxHandshakeBytes > 32<<10 || !hgPowerOfTwo(cfg.RoomShardCount, 16, 4096) || !hgPowerOfTwo(cfg.MemberShardCount, 4, 256) || cfg.MaxPendingBytes < 16<<10 || cfg.MaxPendingBytes > 1<<20 || cfg.CommandRatePerSecond < 1 || cfg.CommandRatePerSecond > 100 || cfg.CommandBurst < cfg.CommandRatePerSecond || cfg.CommandBurst > 500 || !hgPowerOfTwo(cfg.BroadcastWorkerCount, 1, 256) || cfg.BroadcastQueueSize < 64 || cfg.BroadcastQueueSize > 1_000_000 || cfg.RecentMessageLimit < 100 || cfg.RecentMessageLimit > 10_000 {
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

// GetStatisticInfrastructureConfig 读取并校验 ClickHouse 和 Statistic 投影配置。
func GetStatisticInfrastructureConfig() (HGClickHouseConfig, HGStatisticInfrastructureConfig, error) {
	var clickHouse HGClickHouseConfig
	var rawStatistic HGStatisticConfig
	var statistic HGStatisticInfrastructureConfig
	if err := viper.UnmarshalKey("clickhouse", &clickHouse); err != nil {
		return clickHouse, statistic, fmt.Errorf("读取 ClickHouse 配置失败: %w", err)
	}
	if err := viper.UnmarshalKey("statistic", &rawStatistic); err != nil {
		return clickHouse, statistic, fmt.Errorf("读取 Statistic 配置失败: %w", err)
	}
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
