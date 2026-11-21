package module

import (
	"MLC_GO/internal/modules/im/transport"
	Config "MLC_GO/internal/pkg/config"
	Redis "MLC_GO/internal/pkg/redis"
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
	"github.com/redis/go-redis/v9"
)

// 隔离 debug 连接验收：随机不存在用户仅写短 TTL Redis 会话，验证网关代理断开和旧票据拒绝重连。
func TestHGDebugIAMProxyRejectsRevokedSession(t *testing.T) {
	if os.Getenv("MLC_IAM_PROXY_ACCEPTANCE") != "1" {
		t.Skip("未授权写入临时 Redis 测试会话")
	}
	if err := Config.LoadConfig("debug"); err != nil {
		t.Fatal(err)
	}
	cfg, err := HGLoadConfig()
	if err != nil || !cfg.Realtime {
		t.Fatal("debug IAM 网关配置无效")
	}
	endpoint, err := transport.HGEndpoint(cfg.centrifugoURL)
	if err != nil || net.ParseIP(endpoint.Hostname()) == nil || !net.ParseIP(endpoint.Hostname()).IsLoopback() {
		t.Fatal("只允许本机 debug 网关验收")
	}
	wsEndpoint, err := url.Parse(cfg.WebSocketURL)
	if err != nil || (wsEndpoint.Hostname() != "localhost" &&
		(net.ParseIP(wsEndpoint.Hostname()) == nil || !net.ParseIP(wsEndpoint.Hostname()).IsLoopback())) {
		t.Fatal("只允许本机 debug WebSocket 验收")
	}
	redisConfig, err := Config.GetRedisConfig()
	if err != nil || net.ParseIP(redisConfig.Host) == nil || !net.ParseIP(redisConfig.Host).IsLoopback() {
		t.Fatal("只允许本机 debug Redis 验收")
	}
	client := redis.NewClient(&redis.Options{Addr: net.JoinHostPort(redisConfig.Host, redisConfig.Port)})
	defer client.Close()
	gateway, err := transport.HGNewCentrifugo(cfg.centrifugoURL, cfg.centrifugoKey, cfg.centrifugoSecret)
	if err != nil {
		t.Fatal(err)
	}
	defer gateway.HGClose()
	seed := make([]byte, 16)
	if _, err := rand.Read(seed); err != nil {
		t.Fatal(err)
	}
	user, sid := "iam-acceptance-"+hex.EncodeToString(seed), hex.EncodeToString(seed)
	key := Redis.HGAuthSessionKey + user + ":" + sid
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := client.Set(ctx, key, "1", 90*time.Second).Err(); err != nil {
		t.Fatal("无法创建本机临时会话")
	}
	defer client.Del(context.Background(), key)
	grant, err := gateway.HGIAMGrant(user, sid)
	if err != nil {
		t.Fatal(err)
	}
	dial := func() *websocket.Conn {
		t.Helper()
		conn, _, err := (&websocket.Dialer{HandshakeTimeout: 3 * time.Second}).
			Dial(cfg.WebSocketURL, http.Header{"Origin": []string{"http://localhost:5174"}})
		if err != nil {
			t.Fatal("本机 IAM WebSocket 无法握手")
		}
		_ = conn.SetReadDeadline(time.Now().Add(4 * time.Second))
		if err := conn.WriteJSON(map[string]any{"id": 1, "connect": map[string]any{
			"data": map[string]string{"token": grant},
		}}); err != nil {
			t.Fatal("IAM 连接请求失败")
		}
		return conn
	}
	connection := dial()
	defer connection.Close()
	var initial map[string]json.RawMessage
	if err := connection.ReadJSON(&initial); err != nil {
		t.Fatal("有效会话被网关拒绝")
	}
	var subscribed struct {
		Subs map[string]json.RawMessage `json:"subs"` // 只断言本人频道，不读取测试凭据。
	}
	if json.Unmarshal(initial["connect"], &subscribed) != nil ||
		subscribed.Subs[transport.HGAccessChannel(user)] == nil {
		t.Fatal("网关代理没有自动订阅本人频道")
	}
	if err := client.Del(ctx, key).Err(); err != nil {
		t.Fatal("无法撤销测试登录会话")
	}
	if err := gateway.HGDisconnectIAMSession(ctx, user, sid); err != nil {
		t.Fatalf("无法断开被撤销的 Socket: %v", err)
	}
	// 不依赖等待旧连接清理完成：重连必须独立向服务端查 sid，绝不能复用旧票据。
	reconnected := dial()
	defer reconnected.Close()
	var response map[string]json.RawMessage
	if err := reconnected.ReadJSON(&response); err == nil && response["connect"] != nil {
		t.Fatal("旧登录会话成功重连了本人频道")
	}
}
