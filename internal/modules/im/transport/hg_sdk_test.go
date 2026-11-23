package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestHGCentrifugoSDKPublishConfirmsAndBoundsResponse(t *testing.T) {
	for _, tc := range []struct {
		name     string
		response string
		ok       bool
	}{
		{"confirmed", `{"result":{}}`, true},
		{"missing", `{}`, false},
		{"oversized", `{"result":{}}` + strings.Repeat(" ", 65536), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api" || r.Header.Get("X-API-Key") != strings.Repeat("k", 16) || r.Header.Get("Authorization") != "" {
					t.Error("网关鉴权或路径不匹配")
				}
				var command struct {
					Method string `json:"method"`
					Params struct {
						Channel string `json:"channel"`
					} `json:"params"`
				}
				if json.NewDecoder(r.Body).Decode(&command) != nil || command.Method != "publish" || command.Params.Channel != "im:device" {
					t.Error("发布命令不匹配")
				}
				_, _ = w.Write([]byte(tc.response))
			}))
			defer server.Close()
			gateway, err := HGNewCentrifugo(server.URL, strings.Repeat("k", 16), strings.Repeat("s", 32))
			if err != nil {
				t.Fatal(err)
			}
			defer gateway.HGClose()
			err = gateway.HGPublish(context.Background(), "device", json.RawMessage(`{"seq":1}`))
			if (err == nil) != tc.ok {
				t.Fatalf("发布确认状态错误: %v", err)
			}
		})
	}
}

// 用户业务频道兼容既有 IAM 协议，设备私信继续走独立频道。
func TestHGBusinessChannelIsolatedFromDevice(t *testing.T) {
	user := "user-1"
	if HGBusinessChannel(user) != "im:iam:"+user ||
		HGAccessChannel(user) != HGBusinessChannel(user) ||
		HGBusinessChannel(user) == HGDeviceChannel(user) ||
		HGBusinessChannel(user) == HGBusinessChannel("user-2") {
		t.Fatal("业务频道必须兼容现有 IAM 用户频道且隔离私信设备与其他用户")
	}
}

// 服务端签发的用户频道票据仅携带本人业务频道，绝不混入设备私信频道。
func TestHGBusinessTokenHasOnlyUserChannel(t *testing.T) {
	secret := strings.Repeat("s", 48)
	gateway, err := HGNewCentrifugo("http://127.0.0.1:18000", strings.Repeat("k", 16), secret)
	if err != nil {
		t.Fatal(err)
	}
	defer gateway.HGClose()
	raw, err := gateway.HGUserToken("user-1")
	if err != nil {
		t.Fatal(err)
	}
	claims := jwt.MapClaims{}
	parsed, err := jwt.ParseWithClaims(raw, claims, func(*jwt.Token) (any, error) {
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer("mlc-im"), jwt.WithAudience("centrifugo"))
	if err != nil || !parsed.Valid {
		t.Fatalf("业务频道票据无效: %v", err)
	}
	channels, ok := claims["channels"].([]any)
	if !ok || len(channels) != 1 || channels[0] != HGBusinessChannel("user-1") ||
		claims["sub"] != "iam/user-1" {
		t.Fatal("用户业务连接的频道或身份不符合现有协议")
	}
}

// 通知提示复用既有会话鉴权的本人业务频道，不得将通知正文广播到设备或公共频道。
func TestHGCentrifugoUserNotificationHintUsesPrivateChannel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api" || r.Header.Get("X-API-Key") != strings.Repeat("k", 16) {
			t.Error("网关 API 鉴权不匹配")
		}
		var command struct {
			Method string `json:"method"` // Centrifugo 发布方法。
			Params struct {
				Channel string          `json:"channel"` // 仅服务端选择的本人频道。
				Data    json.RawMessage `json:"data"`    // 不含用户或通知正文的提示。
			} `json:"params"`
		}
		if json.NewDecoder(r.Body).Decode(&command) != nil || command.Method != "publish" ||
			command.Params.Channel != HGBusinessChannel("user-1") ||
			string(command.Params.Data) != `{"event_type":"notification.inbox.updated.v1","category":"marketing"}` {
			t.Error("通知提示没有走既有本人私有频道")
		}
		_, _ = w.Write([]byte(`{"result":{}}`))
	}))
	defer server.Close()
	gateway, err := HGNewCentrifugo(server.URL, strings.Repeat("k", 16), strings.Repeat("s", 32))
	if err != nil {
		t.Fatal(err)
	}
	defer gateway.HGClose()
	if err := gateway.HGPublishUser(context.Background(), "user-1",
		json.RawMessage(`{"event_type":"notification.inbox.updated.v1","category":"marketing"}`)); err != nil {
		t.Fatal(err)
	}
}

func TestHGLiveKitSDKJoinTokenHasMinimalGrant(t *testing.T) {
	secret := strings.Repeat("s", 32)
	client, err := HGNewLiveKit("http://127.0.0.1:7880", "fixture", secret)
	if err != nil {
		t.Fatal(err)
	}
	defer client.HGClose()
	token, err := client.HGJoinToken("im_room", "device", "audio")
	if err != nil {
		t.Fatal(err)
	}
	claims := jwt.MapClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(*jwt.Token) (any, error) { return []byte(secret), nil }, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer("fixture"), jwt.WithExpirationRequired())
	if err != nil || !parsed.Valid {
		t.Fatalf("SDK 令牌签名无效: %v", err)
	}
	grant, ok := claims["video"].(map[string]any)
	if !ok || claims["sub"] != "device" || grant["room"] != "im_room" || grant["roomJoin"] != true || grant["canPublishData"] != false || grant["roomRecord"] == true || grant["roomAdmin"] == true {
		t.Fatalf("入房权限超出业务边界: %v", grant)
	}
	if sources, ok := grant["canPublishSources"].([]any); !ok || len(sources) != 1 || sources[0] != "microphone" {
		t.Fatalf("音频发布源越界: %v", grant["canPublishSources"])
	}
	exp, _ := claims.GetExpirationTime()
	if exp == nil || time.Until(exp.Time) > time.Minute || time.Until(exp.Time) < 50*time.Second {
		t.Fatal("入房令牌有效期不符")
	}
}

func TestHGSeaweedFSSDKGetPresignIsPrivateAndBounded(t *testing.T) {
	storage, err := HGNewSeaweedFS("http://127.0.0.1:8333", "us-east-1", "im-private", "fixture", strings.Repeat("s", 32))
	if err != nil {
		t.Fatal(err)
	}
	defer storage.HGClose()
	key := "sealed/" + strings.Repeat("a", 32)
	link, err := storage.hgSign(http.MethodGet, key, "", 0, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(link, "/im-private/"+key) || !strings.Contains(link, "X-Amz-Signature=") || !strings.Contains(link, "X-Amz-Expires=60") || strings.Contains(link, strings.Repeat("s", 32)) {
		t.Fatal("私有对象签名范围或有效期不符")
	}
	if _, err = storage.hgSign(http.MethodGet, "public/file", "", 0, time.Minute); err == nil {
		t.Fatal("不允许签发任意对象")
	}
	grant, err := storage.HGSignRead(key)
	if err != nil || !strings.Contains(grant, "X-Amz-Expires=15") {
		t.Fatal("私有下载凭据必须只有 15 秒有效期")
	}
	if _, err = storage.HGSignRead("quarantine/" + strings.Repeat("a", 32)); err == nil {
		t.Fatal("不得为隔离区或客户端自选对象签发读取凭据")
	}
}

func TestHGSeaweedFSPutPresignDoesNotRequireACL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.Header.Get("x-amz-acl") != "" ||
			r.Header.Get("If-None-Match") != "*" || r.ContentLength != 1 {
			t.Error("封存请求不得携带 ACL，且必须绑定创建条件及长度")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	storage, err := HGNewSeaweedFS(server.URL, "us-east-1", "im-private", "fixture", strings.Repeat("s", 32))
	if err != nil {
		t.Fatal(err)
	}
	defer storage.HGClose()
	key := strings.Repeat("a", 32)
	link, err := storage.HGUploadURL("quarantine/"+key, "text/plain", 1)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(link)
	if err != nil || parsed.Query().Get("X-Amz-SignedHeaders") != "content-length;content-type;host" {
		t.Fatal("上传签名必须绑定长度与内容类型，不得要求 ACL")
	}
	created, err := storage.HGReviewSeal(context.Background(), "sealed/"+key, "text/plain", 1, strings.NewReader("a"))
	if err != nil || !created {
		t.Fatalf("封存请求未按无 ACL 协议发送: %v", err)
	}
}
