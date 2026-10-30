package transport

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"net/http"
	"strings"
	"time"
)

// HGLiveKit 实现 LiveKit JWT 与 RoomService Twirp JSON；房间业务权限由 IM 服务执行。
type HGLiveKit struct {
	endpoint string       // LiveKit HTTP 控制端点。
	key      string       // API Key，同时是 JWT issuer。
	secret   []byte       // JWT 签名密钥，独立于 Centrifugo/业务 JWT。
	client   *http.Client // 有界控制请求池。
}

func HGNewLiveKit(endpoint, key, secret string) (*HGLiveKit, error) {
	if _, err := HGEndpoint(endpoint); err != nil {
		return nil, err
	}
	if key == "" || len(secret) < 32 {
		return nil, fmt.Errorf("LiveKit 凭据未配置")
	}
	return &HGLiveKit{endpoint: strings.TrimRight(endpoint, "/"), key: key, secret: []byte(secret), client: hgClient()}, nil
}
func (l *HGLiveKit) hgToken(identity string, grant map[string]any) (string, error) {
	now := time.Now()
	return jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"iss": l.key, "sub": identity, "nbf": now.Add(-5 * time.Second).Unix(), "exp": now.Add(60 * time.Second).Unix(), "video": grant}).SignedString(l.secret)
}

// HGJoinToken 只允许指定房间及单设备身份；不开放录制、管理、屏幕共享或数据发布。
func (l *HGLiveKit) HGJoinToken(room, identity, kind string) (string, error) {
	sources := []string{"microphone"}
	if kind == "video" {
		sources = append(sources, "camera")
	}
	return l.hgToken(identity, map[string]any{"roomJoin": true, "room": room, "canPublish": true, "canSubscribe": true, "canPublishData": false, "canPublishSources": sources})
}

func (l *HGLiveKit) hgRoom(ctx context.Context, method, room string) error {
	token, err := l.hgToken("", map[string]any{"roomCreate": true})
	if err != nil {
		return err
	}
	request := map[string]any{"room": room}
	if method == "CreateRoom" {
		request = map[string]any{"name": room, "empty_timeout": 60, "departure_timeout": 15, "max_participants": 2}
	}
	return hgPost(ctx, l.client, l.endpoint+"/twirp/livekit.RoomService/"+method, "Bearer "+token, request, nil)
}
func (l *HGLiveKit) HGCreateRoom(ctx context.Context, room string) (string, error) {
	token, err := l.hgToken("", map[string]any{"roomCreate": true})
	if err != nil {
		return "", err
	}
	var result struct {
		SID string `json:"sid"` /* LiveKit 房间实例 ID，不是业务房间名。 */
	}
	err = hgPost(ctx, l.client, l.endpoint+"/twirp/livekit.RoomService/CreateRoom", "Bearer "+token, map[string]any{"name": room, "empty_timeout": 60, "departure_timeout": 15, "max_participants": 2}, &result)
	if err != nil {
		return "", err
	}
	if result.SID == "" || len(result.SID) > 64 {
		return "", fmt.Errorf("LiveKit 未返回房间实例 ID")
	}
	return result.SID, nil
}
func (l *HGLiveKit) HGDeleteRoom(ctx context.Context, room string) error {
	err := l.hgRoom(ctx, "DeleteRoom", room)
	var status hgStatusError
	if errors.As(err, &status) && status.status == http.StatusNotFound {
		return nil
	}
	return err
}
func (l *HGLiveKit) HGClose() {
	if l != nil {
		l.client.CloseIdleConnections()
	}
}

// HGWebhook 是验签后的 LiveKit 最小事件协议；忽略额外厂商字段，不接受客户端伪造状态。
type HGWebhook struct {
	ID    string `json:"id"`    // LiveKit 稳定事件 ID。
	Event string `json:"event"` // 本期处理 room_finished，其他合法事件确认但不改变状态。
	Room  struct {
		SID  string `json:"sid"`  // 实例 ID，防止同名房间重建后的迟到事件。
		Name string `json:"name"` // 服务端生成的 im_<callId>。
	} `json:"room"` // 房间事件目标。
}

// HGVerifyWebhook 限定 HS256/issuer/exp，按 LiveKit 协议常量时间比对原始正文的 Base64 SHA256。
func (l *HGLiveKit) HGVerifyWebhook(header string, body []byte) (HGWebhook, error) {
	var event HGWebhook
	if len(header) > 8192 || len(body) > 65536 {
		return event, fmt.Errorf("LiveKit 回调超限")
	}
	header = strings.TrimPrefix(header, "Bearer ")
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(header, claims, func(*jwt.Token) (any, error) { return l.secret, nil }, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer(l.key), jwt.WithExpirationRequired())
	if err != nil || !token.Valid {
		return event, fmt.Errorf("LiveKit 回调签名无效")
	}
	hash, ok := claims["sha256"].(string)
	sum := sha256.Sum256(body)
	if !ok || subtle.ConstantTimeCompare([]byte(hash), []byte(base64.StdEncoding.EncodeToString(sum[:]))) != 1 {
		return event, fmt.Errorf("LiveKit 回调正文摘要无效")
	}
	if json.Unmarshal(body, &event) != nil || len(event.ID) < 1 || len(event.ID) > 64 {
		return event, fmt.Errorf("LiveKit 回调事件不合法")
	}
	return event, nil
}
