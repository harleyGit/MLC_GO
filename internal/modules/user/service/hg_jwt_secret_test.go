package UserServicePackage

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"
)

func TestHGConfigureJWTSecretRequiresDeploymentKey(t *testing.T) {
	previous := append([]byte(nil), Secret...)
	t.Cleanup(func() { Secret = previous })
	requestKey := append([]byte(nil), HGRequestSignatureKey...)
	t.Cleanup(func() { HGRequestSignatureKey = requestKey })
	t.Setenv("MLC_AUTH_JWT_SECRET_HEX", "")
	if err := HGConfigureJWTSecret("prod"); err == nil {
		t.Fatal("生产环境缺少签名密钥必须失败关闭")
	}
	if err := HGConfigureJWTSecret("pre"); err == nil {
		t.Fatal("预发环境缺少签名密钥必须失败关闭")
	}
	if err := HGConfigureJWTSecret("debug"); err != nil || len(Secret) != 32 || bytes.Equal(Secret, previous) {
		t.Fatalf("开发环境应使用新的临时随机密钥: %v", err)
	}
	t.Setenv("MLC_AUTH_JWT_SECRET_HEX", "invalid")
	if err := HGConfigureJWTSecret("prod"); err == nil || strings.Contains(err.Error(), "invalid") {
		t.Fatal("无效密钥必须失败且不能泄露配置原值")
	}
	key := bytes.Repeat([]byte{0x93}, 32)
	t.Setenv("MLC_AUTH_JWT_SECRET_HEX", hex.EncodeToString(key))
	if err := HGConfigureJWTSecret("prod"); err != nil || !bytes.Equal(Secret, key) {
		t.Fatalf("密钥管理注入的 32 字节值应被加载: %v", err)
	}
	if !bytes.Equal(HGRequestSignatureKey, requestKey) {
		t.Fatal("JWT 私钥更新不得改变浏览器请求签名协议")
	}
	t.Setenv("MLC_API_SIGN_SECRET", "browser-signature-fixture")
	HGConfigureRequestSignatureKey()
	if string(HGRequestSignatureKey) != "browser-signature-fixture" || bytes.Equal(Secret, HGRequestSignatureKey) {
		t.Fatal("浏览器签名密钥不得被用作 JWT 私钥")
	}
}
