package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"net/http"
	"strings"
	"time"
)

// HGCentrifugo 使用 v6 HTTP API 与独立连接 JWT；默认禁止客户端自行订阅/发布。
type HGCentrifugo struct {
	endpoint string       // 服务端 API 根地址，不含 /api。
	key      string       // 仅服务端可见的 API Key。
	secret   []byte       // 独立连接签名密钥，不复用业务 JWT。
	client   *http.Client // 有超时和连接上限的客户端。
}

func HGNewCentrifugo(endpoint, key, secret string) (*HGCentrifugo, error) {
	if _, err := HGEndpoint(endpoint); err != nil {
		return nil, err
	}
	if len(key) < 16 || len(secret) < 32 {
		return nil, fmt.Errorf("Centrifugo 凭据未配置或过短")
	}
	return &HGCentrifugo{endpoint: strings.TrimRight(endpoint, "/"), key: key, secret: []byte(secret), client: hgClient()}, nil
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
	var result struct {
		Result json.RawMessage `json:"result"` // 成功结果对象，空对象也是成功。
		Error  *struct {
			Code int `json:"code"`
		} `json:"error"` // 网关错误代码，不返回内部文本。
	}
	if err := hgPost(ctx, g.client, g.endpoint+"/api/publish", "apikey "+g.key, map[string]any{"channel": HGDeviceChannel(device), "data": payload}, &result); err != nil {
		return err
	}
	var publication map[string]any
	if result.Error != nil || json.Unmarshal(result.Result, &publication) != nil || publication == nil {
		return fmt.Errorf("Centrifugo 未确认发布")
	}
	return nil
}

func (g *HGCentrifugo) HGClose() {
	if g != nil {
		g.client.CloseIdleConnections()
	}
}
