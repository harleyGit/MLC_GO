//go:build production

package main

import (
	IMTransport "MLC_GO/internal/modules/im/transport"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestHGWaitServeErrorsCollectsFailures(t *testing.T) {
	serveErr := make(chan error, 2)
	serveErr <- nil
	serveErr <- errors.New("realtime failed")
	if err := hgWaitServeErrors(serveErr, 2, time.Second); err == nil || !strings.Contains(err.Error(), "realtime failed") {
		t.Fatalf("hgWaitServeErrors() error = %v", err)
	}
}

// 外部能力尚无隔离实例；这些负向协议用例仅使用内存 HTTP 服务，不写数据库或实际网关。
func TestHGIMCentrifugoRejectsUnconfirmedPublish(t *testing.T) {
	for _, hgBody := range []string{`{"error":{"code":100}}`, `{"result":null}`, `{}`, `{"result":[]}`} {
		t.Run(hgBody, func(t *testing.T) {
			hgServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/publish" || r.Header.Get("X-API-Key") != strings.Repeat("k", 16) {
					t.Error("网关请求契约不匹配")
				}
				_, _ = w.Write([]byte(hgBody))
			}))
			defer hgServer.Close()
			hgGateway, hgErr := IMTransport.HGNewCentrifugo(hgServer.URL, strings.Repeat("k", 16), strings.Repeat("s", 32))
			if hgErr != nil {
				t.Fatal(hgErr)
			}
			defer hgGateway.HGClose()
			if hgErr = hgGateway.HGPublish(context.Background(), strings.Repeat("a", 32), json.RawMessage(`{"eventType":"message.committed"}`)); hgErr == nil {
				t.Fatal("HTTP 200 的缺失/失败发布结果被当成成功")
			}
		})
	}
}

func TestHGIMLiveKitWebhookRejectsTamperingAndWrongIdentity(t *testing.T) {
	hgSecret := strings.Repeat("s", 32)
	hgLiveKit, hgErr := IMTransport.HGNewLiveKit("http://127.0.0.1:7880", "fixture", hgSecret)
	if hgErr != nil {
		t.Fatal(hgErr)
	}
	defer hgLiveKit.HGClose()
	hgBody := []byte(`{"id":"event-1","event":"room_finished","room":{"name":"im_01234567890123456789012345678901","sid":"RM_fixture"}}`)
	hgHash := sha256.Sum256(hgBody)
	for _, hgCase := range []struct {
		hgName   string    // 负向条件。
		hgIssuer string    // JWT 签发者。
		hgExpiry time.Time // Token 失效时间。
		hgTamper bool      // 是否更改签名后的正文。
		hgAccept bool      // 是否应通过认证。
	}{
		{"valid", "fixture", time.Now().Add(time.Minute), false, true},
		{"tampered", "fixture", time.Now().Add(time.Minute), true, false},
		{"wrong issuer", "other", time.Now().Add(time.Minute), false, false},
		{"expired", "fixture", time.Now().Add(-time.Minute), false, false},
	} {
		t.Run(hgCase.hgName, func(t *testing.T) {
			hgToken, hgErr := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"iss": hgCase.hgIssuer, "exp": hgCase.hgExpiry.Unix(), "sha256": base64.StdEncoding.EncodeToString(hgHash[:])}).SignedString([]byte(hgSecret))
			if hgErr != nil {
				t.Fatal(hgErr)
			}
			hgPayload := append([]byte(nil), hgBody...)
			if hgCase.hgTamper {
				hgPayload = append(hgPayload, ' ')
			}
			_, hgErr = hgLiveKit.HGVerifyWebhook(hgToken, hgPayload)
			if (hgErr == nil) != hgCase.hgAccept {
				t.Fatalf("验签结果与预期不符: %v", hgErr)
			}
		})
	}
}

func TestHGIMPrivateUploadCannotSignSealedOrUnboundedObjects(t *testing.T) {
	hgStorage, hgErr := IMTransport.HGNewSeaweedFS("http://127.0.0.1:8333", "us-east-1", "im-private", "fixture", strings.Repeat("s", 32))
	if hgErr != nil {
		t.Fatal(hgErr)
	}
	defer hgStorage.HGClose()
	for _, hgKey := range []string{"sealed/" + strings.Repeat("a", 32), "quarantine/../sealed/" + strings.Repeat("a", 32), "quarantine/a?acl=public-read"} {
		if _, hgErr = hgStorage.HGUploadURL(hgKey, "image/png", 10); hgErr == nil {
			t.Fatal("越过隔离对象路径边界")
		}
	}
	if _, hgErr = hgStorage.HGUploadURL("quarantine/"+strings.Repeat("a", 32), "image/png", 129<<20); hgErr == nil {
		t.Fatal("未拒绝超限上传")
	}
	hgLink, hgErr := hgStorage.HGUploadURL("quarantine/"+strings.Repeat("a", 32), "image/png", 10)
	if hgErr != nil {
		t.Fatal(hgErr)
	}
	hgURL, hgErr := url.Parse(hgLink)
	if hgErr != nil {
		t.Fatal(hgErr)
	}
	if hgURL.Query().Get("X-Amz-SignedHeaders") != "content-length;content-type;host;x-amz-acl" {
		t.Fatal("上传未签名大小/类型/私有 ACL")
	}
	if hgURL.Query().Get("X-Amz-Expires") != "600" {
		t.Fatal("上传授权有效期不符")
	}
}

func TestHGWaitServeErrorsHasBoundedTimeout(t *testing.T) {
	serveErr := make(chan error)
	startedAt := time.Now()
	err := hgWaitServeErrors(serveErr, 2, 20*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "2 server listeners did not stop") {
		t.Fatalf("hgWaitServeErrors() timeout error = %v", err)
	}
	if elapsed := time.Since(startedAt); elapsed > time.Second {
		t.Fatalf("hgWaitServeErrors() blocked for %s", elapsed)
	}
}
