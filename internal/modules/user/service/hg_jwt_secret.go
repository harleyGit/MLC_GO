package UserServicePackage

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"strings"
)

// HGRequestSignatureKey 是与浏览器 VITE_SIGN_SECRET 对齐的公开请求签名材料；
// 不得用作 JWT 身份认证密钥，默认值保留旧版浏览器请求协议的兼容性。
var HGRequestSignatureKey = []byte("change-me")

// hgEphemeralJWTSecret 仅用于无配置的开发进程，重启后所有本机令牌失效。
func hgEphemeralJWTSecret() []byte {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic("无法生成开发环境 JWT 签名密钥")
	}
	return key
}

// HGConfigureJWTSecret 启动期加载 32 字节签名密钥；pre/prod 缺失时拒绝启动，不回退到公开常量。
// 环境变量 MLC_AUTH_JWT_SECRET_HEX 必须由部署密钥管理注入，禁止打印、提交或通过接口返回。
func HGConfigureJWTSecret(env string) error {
	if env != "debug" && env != "pre" && env != "prod" {
		return errors.New("不支持的 JWT 运行环境")
	}
	raw := strings.TrimSpace(os.Getenv("MLC_AUTH_JWT_SECRET_HEX"))
	if raw == "" {
		if env == "debug" {
			Secret = hgEphemeralJWTSecret()
			return nil
		}
		return errors.New("JWT 签名密钥未配置")
	}
	key, err := hex.DecodeString(raw)
	if err != nil || len(key) != 32 {
		return errors.New("JWT 签名密钥格式无效，要求 32 字节十六进制")
	}
	Secret = key
	return nil
}

// HGConfigureRequestSignatureKey 启动期匹配浏览器的 VITE_SIGN_SECRET；不承载 JWT 身份鉴别。
func HGConfigureRequestSignatureKey() {
	if configured := os.Getenv("MLC_API_SIGN_SECRET"); configured != "" {
		HGRequestSignatureKey = []byte(configured)
	}
}
