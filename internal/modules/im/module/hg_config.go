package module

import (
	"MLC_GO/internal/modules/im/transport"
	Config "MLC_GO/internal/pkg/config"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// HGConfig 由现有环境加载机制注入；外部能力默认关闭，凭据只读服务端环境变量。
type HGConfig struct {
	Realtime         bool   // 实时消费和后台重试开关。
	Media            bool   // 私有媒体开关，依赖审核器。
	Calls            bool   // LiveKit 单聊呼叫开关。
	Topic            string // 独立 IM topic，不能与现有业务 topic 共用。
	Group            string // 独立消费组 ID。
	WebSocketURL     string // Centrifugo 客户端端点。
	LiveKitURL       string // LiveKit 客户端端点。
	centrifugoURL    string // 服务端 HTTP API 根地址。
	centrifugoKey    string // 服务端 API Key。
	centrifugoSecret string // 专用连接 JWT 密钥。
	s3URL            string // SeaweedFS S3 端点。
	s3Region         string // SigV4 region。
	s3Bucket         string // 私有桶。
	s3Key            string // S3 Access Key。
	s3Secret         string // S3 Secret Key。
	reviewURL        string // 审核 API。
	reviewToken      string // 审核 API 独立密钥。
	livekitURL       string // LiveKit 控制 API。
	livekitKey       string // LiveKit API Key。
	livekitSecret    string // LiveKit API Secret。
}

// HGLoadConfig 只做静态校验；pre/prod 仍不可启用，外部设施验收后需单独评审解除门禁。
func HGLoadConfig() (HGConfig, error) {
	var cfg HGConfig
	for name, target := range map[string]*bool{"MLC_IM_REALTIME_ENABLED": &cfg.Realtime, "MLC_IM_MEDIA_ENABLED": &cfg.Media, "MLC_IM_CALL_ENABLED": &cfg.Calls} {
		if raw := os.Getenv(name); raw != "" {
			value, err := strconv.ParseBool(raw)
			if err != nil {
				return cfg, fmt.Errorf("%s 必须为布尔值", name)
			}
			*target = value
		}
	}
	if !cfg.Realtime && (cfg.Media || cfg.Calls) {
		return cfg, fmt.Errorf("IM 媒体和通话必须同时启用实时模块")
	}
	if !cfg.Realtime {
		return cfg, nil
	}
	if Config.GetEnv() != Config.EnvDebug {
		return cfg, fmt.Errorf("IM 扩展仅允许隔离 debug 配置，pre/prod 尚未验收")
	}
	cfg.Topic = os.Getenv("MLC_IM_TOPIC")
	cfg.Group = os.Getenv("MLC_IM_CONSUMER_GROUP")
	if !regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,248}$`).MatchString(cfg.Topic) || cfg.Group == "" || len(cfg.Group) > 200 {
		return cfg, fmt.Errorf("IM 专用 topic 和消费组必须显式配置")
	}
	cfg.centrifugoURL = os.Getenv("MLC_IM_CENTRIFUGO_API_URL")
	cfg.centrifugoKey = os.Getenv("MLC_IM_CENTRIFUGO_API_KEY")
	cfg.centrifugoSecret = os.Getenv("MLC_IM_CENTRIFUGO_TOKEN_SECRET")
	cfg.WebSocketURL = os.Getenv("MLC_IM_CENTRIFUGO_WS_URL")
	if err := hgWebSocketURL(cfg.WebSocketURL); err != nil {
		return cfg, err
	}
	if _, err := transport.HGEndpoint(cfg.centrifugoURL); err != nil {
		return cfg, err
	}
	if len(cfg.centrifugoKey) < 16 || len(cfg.centrifugoSecret) < 32 {
		return cfg, fmt.Errorf("Centrifugo 独立密钥缺失")
	}
	if cfg.Media {
		cfg.s3URL = os.Getenv("MLC_IM_S3_ENDPOINT")
		cfg.s3Region = os.Getenv("MLC_IM_S3_REGION")
		cfg.s3Bucket = os.Getenv("MLC_IM_S3_BUCKET")
		cfg.s3Key = os.Getenv("MLC_IM_S3_ACCESS_KEY")
		cfg.s3Secret = os.Getenv("MLC_IM_S3_SECRET_KEY")
		cfg.reviewURL = os.Getenv("MLC_IM_REVIEW_URL")
		cfg.reviewToken = os.Getenv("MLC_IM_REVIEW_TOKEN")
		if _, err := transport.HGEndpoint(cfg.s3URL); err != nil {
			return cfg, err
		}
		if _, err := transport.HGEndpoint(cfg.reviewURL); err != nil {
			return cfg, err
		}
		if cfg.s3Region == "" || cfg.s3Bucket == "" || cfg.s3Key == "" || cfg.s3Secret == "" || len(cfg.reviewToken) < 32 {
			return cfg, fmt.Errorf("IM 私有存储或审核配置不完整")
		}
	}
	if cfg.Calls {
		cfg.livekitURL = os.Getenv("MLC_IM_LIVEKIT_API_URL")
		cfg.LiveKitURL = os.Getenv("MLC_IM_LIVEKIT_WS_URL")
		cfg.livekitKey = os.Getenv("MLC_IM_LIVEKIT_API_KEY")
		cfg.livekitSecret = os.Getenv("MLC_IM_LIVEKIT_API_SECRET")
		if _, err := transport.HGEndpoint(cfg.livekitURL); err != nil {
			return cfg, err
		}
		if err := hgWebSocketURL(cfg.LiveKitURL); err != nil {
			return cfg, err
		}
		if cfg.livekitKey == "" || len(cfg.livekitSecret) < 32 || cfg.livekitSecret == cfg.centrifugoSecret {
			return cfg, fmt.Errorf("LiveKit 独立密钥缺失或与网关共用")
		}
	}
	return cfg, nil
}

func hgWebSocketURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "ws" && u.Scheme != "wss" {
		return fmt.Errorf("IM 客户端 WebSocket 地址不合法")
	}
	u.Scheme = strings.Replace(u.Scheme, "ws", "http", 1)
	_, err = transport.HGEndpoint(u.String())
	return err
}
