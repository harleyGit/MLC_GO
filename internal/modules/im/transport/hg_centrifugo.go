package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/centrifugal/gocent/v3"
	"github.com/golang-jwt/jwt/v5"
)

// HGCentrifugo 使用 v6 HTTP API 与独立连接 JWT；默认禁止客户端自行订阅/发布。
type HGCentrifugo struct {
	secret []byte         // 独立连接签名密钥，不复用业务 JWT。
	client *http.Client   // 有超时和连接上限的客户端。
	sdk    *gocent.Client // Centrifugo 服务端 API 客户端，仅发布已授权设备频道。
}

func HGNewCentrifugo(endpoint, key, secret string) (*HGCentrifugo, error) {
	if _, err := HGEndpoint(endpoint); err != nil {
		return nil, err
	}
	if len(key) < 16 || len(secret) < 32 {
		return nil, fmt.Errorf("Centrifugo 凭据未配置或过短")
	}
	client := hgClient()
	// gocent 默认将凭据放入 Authorization；网关现有策略只接受 X-API-Key。
	client.Transport = hgCentrifugoTransport{base: client.Transport}
	addr := strings.TrimRight(endpoint, "/")
	return &HGCentrifugo{secret: []byte(secret), client: client, sdk: gocent.New(gocent.Config{Addr: addr + "/api", Key: key, HTTPClient: client})}, nil
}

// HGDeviceChannel 仅由服务端已认证的十六进制设备 ID 构造，客户端无法选择他人频道。
func HGDeviceChannel(device string) string { return "im:" + device }

// HGToken 自动订阅单设备频道；exp 限制撤销凭据后的连接残留窗口为 60 秒加网关宽限。
func (g *HGCentrifugo) HGToken(device string) (string, error) {
	now := time.Now()
	return jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "im/" + device, "iss": "mlc-im", "aud": "centrifugo", "iat": now.Unix(), "exp": now.Add(time.Minute).Unix(),
		"channels": []string{HGDeviceChannel(device)},
	}).SignedString(g.secret)
}

// HGPublish 只确认网关受理；不能修改设备 delivered_at。
func (g *HGCentrifugo) HGPublish(ctx context.Context, device string, payload json.RawMessage) error {
	pipe := g.sdk.Pipe()
	if err := pipe.AddPublish(HGDeviceChannel(device), payload); err != nil {
		return fmt.Errorf("Centrifugo 发布参数无效")
	}
	result, err := g.sdk.SendPipe(ctx, pipe)
	if err != nil || len(result) != 1 || result[0].Error != nil {
		return fmt.Errorf("Centrifugo 未确认发布")
	}
	var publication map[string]any
	if json.Unmarshal(result[0].Result, &publication) != nil || publication == nil {
		return fmt.Errorf("Centrifugo 未确认发布")
	}
	return nil
}

// HGUserToken 为服务端认证的业务用户自动订阅 IAM 私有频道；前端无法指定其他用户频道。
func (g *HGCentrifugo) HGUserToken(userID string) (string, error) {
	now := time.Now()
	return jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "iam/" + userID, "iss": "mlc-im", "aud": "centrifugo", "iat": now.Unix(), "exp": now.Add(time.Minute).Unix(),
		"channels": []string{HGAccessChannel(userID)},
	}).SignedString(g.secret)
}

// HGAccessChannel 复用网关已部署的 im 命名空间；iam: 前缀与 32 位十六进制设备频道互斥。
func HGAccessChannel(userID string) string { return "im:iam:" + userID }

// HGIAMGrant 只供网关 connect proxy 校验；它不是 Centrifugo 连接 JWT，
// 单次获取后可用于重连，实际生效仍依赖服务端逐次校验登录会话。
func (g *HGCentrifugo) HGIAMGrant(userID, sessionID string) (string, error) {
	if userID == "" || len(sessionID) != 32 {
		return "", fmt.Errorf("IAM 登录会话无效")
	}
	now := time.Now()
	return jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": userID, "sid": sessionID, "iss": "mlc-iam-proxy", "aud": "mlc-im-iam",
		"iat": now.Unix(), "exp": now.Add(7 * 24 * time.Hour).Unix(),
	}).SignedString(g.secret)
}

// HGVerifyIAMGrant 只验签和协议字段；调用方必须再次查权威登录会话是否仍有效。
func (g *HGCentrifugo) HGVerifyIAMGrant(raw string) (string, string, error) {
	claims := jwt.MapClaims{}
	token, err := jwt.NewParser(jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}),
		jwt.WithIssuer("mlc-iam-proxy"), jwt.WithAudience("mlc-im-iam"), jwt.WithExpirationRequired()).
		ParseWithClaims(raw, claims, func(*jwt.Token) (any, error) { return g.secret, nil })
	if err != nil || !token.Valid {
		return "", "", fmt.Errorf("IAM 连接凭据无效")
	}
	user, _ := claims["sub"].(string)
	sid, _ := claims["sid"].(string)
	if user == "" || len(sid) != 32 {
		return "", "", fmt.Errorf("IAM 连接会话无效")
	}
	return user, sid, nil
}

// HGDisconnectIAMSession 精确断开此登录会话的 Socket，其他设备的本人频道不受影响。
func (g *HGCentrifugo) HGDisconnectIAMSession(ctx context.Context, userID, sessionID string) error {
	if userID == "" || len(sessionID) != 32 {
		return fmt.Errorf("IAM 登录会话无效")
	}
	if err := g.sdk.Disconnect(ctx, "iam/"+userID+"/"+sessionID); err != nil {
		return fmt.Errorf("IAM 网关断开连接失败（%T）", err)
	}
	return nil
}

// HGPublishUser 发布服务端选择的本人频道事件（权限快照或无正文通知提示）；网关确认不是设备 ACK。
func (g *HGCentrifugo) HGPublishUser(ctx context.Context, userID string, payload json.RawMessage) error {
	pipe := g.sdk.Pipe()
	if err := pipe.AddPublish(HGAccessChannel(userID), payload); err != nil {
		return fmt.Errorf("IAM 实时发布参数无效")
	}
	result, err := g.sdk.SendPipe(ctx, pipe)
	if err != nil || len(result) != 1 || result[0].Error != nil {
		return fmt.Errorf("IAM 实时网关未确认发布")
	}
	var publication map[string]any
	if json.Unmarshal(result[0].Result, &publication) != nil || publication == nil {
		return fmt.Errorf("IAM 实时网关未确认发布")
	}
	return nil
}

// hgCentrifugoTransport 保持原网关凭据头及 64KiB 响应上限，不让 SDK 回显外部正文。
type hgCentrifugoTransport struct{ base http.RoundTripper }

func (t hgCentrifugoTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	copy := req.Clone(req.Context())
	if auth := copy.Header.Get("Authorization"); strings.HasPrefix(auth, "apikey ") {
		copy.Header.Set("X-API-Key", strings.TrimPrefix(auth, "apikey "))
		copy.Header.Del("Authorization")
	}
	resp, err := t.base.RoundTrip(copy)
	if err != nil {
		return nil, err
	}
	data, readErr := io.ReadAll(io.LimitReader(resp.Body, 65537))
	_ = resp.Body.Close()
	if readErr != nil || len(data) > 65536 {
		return nil, fmt.Errorf("Centrifugo 响应超限或读取失败")
	}
	resp.Body = io.NopCloser(bytes.NewReader(data))
	return resp, nil
}

func (t hgCentrifugoTransport) CloseIdleConnections() {
	if closer, ok := t.base.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
}

func (g *HGCentrifugo) HGClose() {
	if g != nil {
		g.client.CloseIdleConnections()
	}
}
