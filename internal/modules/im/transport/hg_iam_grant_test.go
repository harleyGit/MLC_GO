package transport

import (
	"strings"
	"testing"
)

// 本人会话票据与设备 JWT 不同；改写身份/会话后必须验签失败。
func TestHGIAMGrantBoundToSession(t *testing.T) {
	gateway, err := HGNewCentrifugo("http://127.0.0.1:18000",
		strings.Repeat("a", 24), strings.Repeat("b", 48))
	if err != nil {
		t.Fatal(err)
	}
	defer gateway.HGClose()
	token, err := gateway.HGIAMGrant("U1", strings.Repeat("c", 32))
	if err != nil {
		t.Fatal(err)
	}
	user, sid, err := gateway.HGVerifyIAMGrant(token)
	if err != nil || user != "U1" || sid != strings.Repeat("c", 32) {
		t.Fatalf("IAM 票据主体或会话无效: %v", err)
	}
	if _, _, err := gateway.HGVerifyIAMGrant(token + "tampered"); err == nil {
		t.Fatal("篡改票据不得通过网关连接校验")
	}
	if _, err := gateway.HGIAMGrant("U1", "invalid-session"); err == nil {
		t.Fatal("无会话 ID 不得签发可重连票据")
	}
}
