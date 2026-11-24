package module

import (
	IAM "MLC_GO/internal/modules/iam"
	"MLC_GO/internal/modules/im/transport"
	UserJWT "MLC_GO/internal/modules/user/middleware"
	UserService "MLC_GO/internal/modules/user/service"
	Config "MLC_GO/internal/pkg/config"
	Queries "MLC_GO/internal/pkg/mysql/queries"
	Redis "MLC_GO/internal/pkg/redis"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/golang-jwt/jwt/v5"
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
	secondSeed := make([]byte, 16)
	if _, err := rand.Read(secondSeed); err != nil {
		t.Fatal(err)
	}
	secondSID := hex.EncodeToString(secondSeed)
	secondKey := Redis.HGAuthSessionKey + user + ":" + secondSID
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := client.Set(ctx, key, "1", 90*time.Second).Err(); err != nil {
		t.Fatal("无法创建本机临时会话")
	}
	defer client.Del(context.Background(), key)
	if err := client.Set(ctx, secondKey, "1", 90*time.Second).Err(); err != nil {
		t.Fatal("无法创建同用户第二登录会话")
	}
	defer client.Del(context.Background(), secondKey)
	grant, err := gateway.HGIAMGrant(user, sid)
	if err != nil {
		t.Fatal(err)
	}
	secondGrant, err := gateway.HGIAMGrant(user, secondSID)
	if err != nil {
		t.Fatal(err)
	}
	dial := func(token string) *websocket.Conn {
		t.Helper()
		conn, _, err := (&websocket.Dialer{HandshakeTimeout: 3 * time.Second}).
			Dial(cfg.WebSocketURL, http.Header{"Origin": []string{"http://localhost:5174"}})
		if err != nil {
			t.Fatal("本机 IAM WebSocket 无法握手")
		}
		_ = conn.SetReadDeadline(time.Now().Add(4 * time.Second))
		if err := conn.WriteJSON(map[string]any{"id": 1, "connect": map[string]any{
			"data": map[string]string{"token": token},
		}}); err != nil {
			t.Fatal("IAM 连接请求失败")
		}
		return conn
	}
	connection := dial(grant)
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
	otherSession := dial(secondGrant)
	defer otherSession.Close()
	var otherInitial map[string]json.RawMessage
	if err := otherSession.ReadJSON(&otherInitial); err != nil || otherInitial["connect"] == nil {
		t.Fatal("同用户另一个有效登录会话不能连接")
	}
	// 浏览器 SDK 持续读取 Socket；测试也必须消费关闭帧，避免模拟出不会读取的僵尸客户端。
	closed := make(chan error, 1)
	go func() {
		_ = connection.SetReadDeadline(time.Now().Add(6 * time.Second))
		for {
			if _, _, err := connection.ReadMessage(); err != nil {
				closed <- err
				return
			}
		}
	}()
	originalRedis := Redis.RDB
	Redis.RDB = client
	defer func() { Redis.RDB = originalRedis }()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	UserService.HGConfigureSessionDB(db)
	defer UserService.HGConfigureSessionDB(nil)
	sessionBytes, _ := hex.DecodeString(sid)
	mock.ExpectQuery(regexp.QuoteMeta(Queries.HGSelectRecoveryRevokedAt)).WithArgs(user).
		WillReturnRows(sqlmock.NewRows([]string{"mfa_recovery_revoked_at"}).AddRow(nil))
	mock.ExpectQuery(regexp.QuoteMeta(Queries.HGIMForcedLogoutSQL)).
		WithArgs(user, sessionBytes).
		WillReturnError(sql.ErrNoRows)
	service := IAM.NewHGService(IAM.NewHGRepository(nil))
	service.HGWithRealtime(gateway, cfg.WebSocketURL)
	handler := UserJWT.AuthMiddleware(http.HandlerFunc(IAM.NewHGHandler(service).HGLogoutRealtime))
	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, &UserService.HGClaims{
		UserID: user, SessionID: sid, TokenTp: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
			Issuer:    "mlc-go",
			Subject:   "user-token",
		},
	}).SignedString(UserService.Secret)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/iam/me/logout", nil)
	request.Header.Set("Authorization", "Bearer "+accessToken)
	result := httptest.NewRecorder()
	handler.ServeHTTP(result, request)
	if result.Code != http.StatusOK {
		t.Fatalf("登出服务端未确认撤销和断连: HTTP %d", result.Code)
	}
	if present, err := client.Exists(ctx, key).Result(); err != nil || present != 0 {
		t.Fatal("当前登录会话仍保留在权威 Redis 中")
	}
	if present, err := client.Exists(ctx, secondKey).Result(); err != nil || present != 1 {
		t.Fatal("其他登录会话不应被撤销")
	}
	result = httptest.NewRecorder()
	handler.ServeHTTP(result, request)
	if result.Code != http.StatusUnauthorized {
		t.Fatalf("已撤销的 access token 不得再次通过鉴权: HTTP %d", result.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-closed:
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			t.Fatal("旧 Socket 没有被网关关闭")
		}
	case <-time.After(6 * time.Second):
		t.Fatal("等待旧 Socket 断开超时")
	}
	if err := gateway.HGPublishUser(ctx, user, json.RawMessage(`{"acceptance":"other-session"}`)); err != nil {
		t.Fatal("无法发布同用户测试事件")
	}
	var otherPublication struct {
		Push struct {
			Pub struct {
				Data struct {
					Acceptance string `json:"acceptance"` // 第二会话仍能收到本人频道的测试事件。
				} `json:"data"`
			} `json:"pub"`
		} `json:"push"`
	}
	if err := otherSession.ReadJSON(&otherPublication); err != nil ||
		otherPublication.Push.Pub.Data.Acceptance != "other-session" {
		t.Fatal("撤销第一个会话不应断开或影响同用户其他会话")
	}
	// 不依赖等待旧连接清理完成：重连必须独立向服务端查 sid，绝不能复用旧票据。
	reconnected := dial(grant)
	defer reconnected.Close()
	var response map[string]json.RawMessage
	if err := reconnected.ReadJSON(&response); err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			t.Fatal("旧登录票据重连没有得到明确拒绝")
		}
	} else if response["connect"] != nil || response["disconnect"] == nil && response["error"] == nil {
		t.Fatal("旧登录会话成功重连了本人频道")
	}
}
