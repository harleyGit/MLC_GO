package module

import (
	"MLC_GO/internal/modules/im/transport"
	Config "MLC_GO/internal/pkg/config"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestHGDebugIAMGatewayPublish 只在显式验收时向随机且不存在的本人频道发协议探针，不写用户或通知。
func TestHGDebugIAMGatewayPublish(t *testing.T) {
	if os.Getenv("MLC_IAM_REALTIME_INTEGRATION") != "1" {
		t.Skip("未启用本机 debug IAM 网关验收")
	}
	if err := Config.LoadConfig("debug"); err != nil {
		t.Fatal(err)
	}
	cfg, err := HGLoadConfig()
	if err != nil || !cfg.Realtime {
		t.Fatal("本机 debug 即时通讯未启用")
	}
	endpoint, err := transport.HGEndpoint(cfg.centrifugoURL)
	if err != nil || net.ParseIP(endpoint.Hostname()) == nil || !net.ParseIP(endpoint.Hostname()).IsLoopback() {
		t.Fatal("只允许本机 debug 网关验收")
	}
	gateway, err := transport.HGNewCentrifugo(cfg.centrifugoURL, cfg.centrifugoKey, cfg.centrifugoSecret)
	if err != nil {
		t.Fatal("IAM 网关无法初始化")
	}
	defer gateway.HGClose()
	random := make([]byte, 16)
	if _, err = rand.Read(random); err != nil {
		t.Fatal(err)
	}
	owner := "iam-acceptance-" + hex.EncodeToString(random)
	token, err := gateway.HGUserToken(owner)
	if err != nil || token == "" {
		t.Fatal("本人频道签发失败")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	if err := gateway.HGPublishUser(ctx, owner, []byte(`{"event_type":"iam.acceptance.v1"}`)); err != nil {
		t.Fatal("IAM 网关发布失败")
	}
	wsURL, err := url.Parse(cfg.WebSocketURL)
	if err != nil || (wsURL.Hostname() != "localhost" &&
		(net.ParseIP(wsURL.Hostname()) == nil || !net.ParseIP(wsURL.Hostname()).IsLoopback())) {
		t.Fatal("只允许本机 WebSocket 验收")
	}
	connection, _, err := (&websocket.Dialer{HandshakeTimeout: 3 * time.Second}).
		Dial(cfg.WebSocketURL, http.Header{"Origin": []string{"http://localhost:5174"}})
	if err != nil {
		t.Fatal("IAM 本人频道握手失败")
	}
	defer connection.Close()
	_ = connection.SetReadDeadline(time.Now().Add(4 * time.Second))
	if err := connection.WriteJSON(map[string]any{"id": 1, "connect": map[string]string{"token": token}}); err != nil {
		t.Fatal("IAM 网关连接请求失败")
	}
	connected := false
	for attempt := 0; attempt < 5; attempt++ {
		var message map[string]json.RawMessage
		if err := connection.ReadJSON(&message); err != nil {
			t.Fatal("IAM 网关拒绝本人订阅")
		}
		if response, ok := message["connect"]; ok {
			var subscription struct {
				Subs map[string]json.RawMessage `json:"subs"` // 仅检查频道名称，绝不打印连接凭据。
			}
			if json.Unmarshal(response, &subscription) != nil ||
				subscription.Subs[transport.HGAccessChannel(owner)] == nil {
				t.Fatal("IAM 私有频道未自动订阅")
			}
			connected = true
			break
		}
	}
	if !connected {
		t.Fatal("IAM 频道未完成自动订阅")
	}
	if err := gateway.HGPublishUser(ctx, owner, []byte(`{"event_type":"iam.acceptance.v1"}`)); err != nil {
		t.Fatal("IAM 已连接频道发布失败")
	}
	received := false
	for attempt := 0; attempt < 5; attempt++ {
		var message struct {
			Push struct {
				Channel string `json:"channel"` // 网关 Push 信封中的频道；不得误读顶层 Reply。
				Pub     struct {
					Data struct {
						EventType string `json:"event_type"` // 测试事件，不包含真实用户权限。
					} `json:"data"`
				} `json:"pub"`
			} `json:"push"`
		}
		if err := connection.ReadJSON(&message); err != nil {
			t.Fatal("IAM 已订阅频道未收到实时投递")
		}
		if message.Push.Channel == transport.HGAccessChannel(owner) && message.Push.Pub.Data.EventType == "iam.acceptance.v1" {
			received = true
			break
		}
	}
	if !received {
		t.Fatal("IAM 实时投递未送达目标用户频道")
	}
}
