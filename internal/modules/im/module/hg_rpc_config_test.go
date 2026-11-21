package module

import (
	"strings"
	"testing"
)

func TestHGLoadConfigRequiresIndependentRPCSecret(t *testing.T) {
	t.Setenv("SERVER_ENV", "debug")
	t.Setenv("MLC_IM_REALTIME_ENABLED", "true")
	t.Setenv("MLC_IM_MEDIA_ENABLED", "false")
	t.Setenv("MLC_IM_CALL_ENABLED", "false")
	t.Setenv("MLC_IM_TOPIC", "mlc.im.debug.test")
	t.Setenv("MLC_IM_CONSUMER_GROUP", "im-debug-test")
	t.Setenv("MLC_IM_CENTRIFUGO_API_URL", "http://127.0.0.1:18000")
	t.Setenv("MLC_IM_CENTRIFUGO_API_KEY", strings.Repeat("a", 24))
	t.Setenv("MLC_IM_CENTRIFUGO_TOKEN_SECRET", strings.Repeat("b", 48))
	t.Setenv("MLC_IM_CENTRIFUGO_WS_URL", "ws://127.0.0.1:18000/connection/websocket")
	for _, tc := range []struct {
		name   string
		secret string
		valid  bool
	}{
		{name: "缺失", secret: ""},
		{name: "复用连接密钥", secret: strings.Repeat("b", 48)},
		{name: "独立密钥", secret: strings.Repeat("c", 48), valid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("MLC_IM_RPC_PROXY_SECRET", tc.secret)
			_, err := HGLoadConfig()
			if (err == nil) != tc.valid {
				t.Fatalf("RPC 代理密钥校验结果不符: %v", err)
			}
		})
	}
}

// 非 debug 复用既有实时开关，但不接受聊天媒体/通话，也不要求聊天 Kafka/RPC 凭据。
func TestHGLoadConfigPreIAMOnly(t *testing.T) {
	t.Setenv("SERVER_ENV", "pre")
	t.Setenv("MLC_IM_REALTIME_ENABLED", "true")
	t.Setenv("MLC_IM_MEDIA_ENABLED", "false")
	t.Setenv("MLC_IM_CALL_ENABLED", "false")
	t.Setenv("MLC_IM_TOPIC", "")
	t.Setenv("MLC_IM_CONSUMER_GROUP", "")
	t.Setenv("MLC_IM_RPC_PROXY_SECRET", "")
	t.Setenv("MLC_IM_CENTRIFUGO_API_URL", "https://gateway.example.com")
	t.Setenv("MLC_IM_CENTRIFUGO_API_KEY", strings.Repeat("a", 24))
	t.Setenv("MLC_IM_CENTRIFUGO_TOKEN_SECRET", strings.Repeat("b", 48))
	t.Setenv("MLC_IM_CENTRIFUGO_WS_URL", "wss://gateway.example.com/connection/websocket")
	cfg, err := HGLoadConfig()
	if err != nil || !cfg.Realtime || !cfg.IAMOnly || cfg.Topic != "" || cfg.Group != "" {
		t.Fatalf("pre 环境应仅启用 IAM 本人频道: %v", err)
	}
	t.Setenv("MLC_IM_MEDIA_ENABLED", "true")
	if _, err := HGLoadConfig(); err == nil {
		t.Fatal("pre 环境不得启用未验收的聊天媒体")
	}
	t.Setenv("MLC_IM_MEDIA_ENABLED", "false")
	t.Setenv("MLC_IM_CENTRIFUGO_API_KEY", "")
	if _, err := HGLoadConfig(); err == nil {
		t.Fatal("缺少网关凭据应拒绝启用 IAM 实时通道")
	}
}
