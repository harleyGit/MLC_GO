package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
}
