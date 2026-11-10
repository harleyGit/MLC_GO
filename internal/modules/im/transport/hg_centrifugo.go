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
