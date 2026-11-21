package module

import (
	"MLC_GO/internal/modules/im/transport"
	Redis "MLC_GO/internal/pkg/redis"
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 网关私有头不能由客户端省略，票据即使验签通过但会话存储不可用也必须拒绝。
func TestHGIAMConnectFailsClosed(t *testing.T) {
	gateway, err := transport.HGNewCentrifugo("http://127.0.0.1:18000",
		strings.Repeat("a", 24), strings.Repeat("b", 48))
	if err != nil {
		t.Fatal(err)
	}
	defer gateway.HGClose()
	grant, err := gateway.HGIAMGrant("U1", strings.Repeat("a", 32))
	if err != nil {
		t.Fatal(err)
	}
	handler := hgIAMConnectHandler(gateway, strings.Repeat("c", 48))
	payload := []byte(`{"data":{"token":"` + grant + `"}}`)
	for _, tc := range []struct {
		name   string
		secret string
		status int
		body   string
	}{
		{"没有可信网关凭据", "", 403, ""},
		{"会话存储不可用", strings.Repeat("c", 48), 200, `"disconnect"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/internal/im/centrifugo/iam-connect", bytes.NewReader(payload))
			request.Header.Set("X-IM-RPC-Secret", tc.secret)
			result := httptest.NewRecorder()
			// nil Redis 必须失败关闭；保存原有全局依赖供其他测试使用。
			previous := Redis.RDB
			Redis.RDB = nil
			defer func() { Redis.RDB = previous }()
			handler.ServeHTTP(result, request)
			if result.Code != tc.status || tc.body != "" && !strings.Contains(result.Body.String(), tc.body) {
				t.Fatalf("拒绝连接结果不正确: HTTP %d", result.Code)
			}
		})
	}
}
