/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-23 13:49:26
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-04-18 03:08:00
 * @FilePath: /MLC_GO/internal/pkg/device/hg_fingerprint.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package PkGDevicePackage

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/http"
	"strings"
)

/* 设备指纹 + 风控升级 */
// Fingerprint 生成请求设备指纹：优先使用稳定的 X-Device-ID，避免因代理源端口变化导致误判。
func Fingerprint(r *http.Request) string {
	if r == nil {
		return hashFingerprint("unknown-device")
	}

	deviceID := normalizeHeaderValue(r.Header.Get("X-Device-ID"))
	clientType := normalizeHeaderValue(r.Header.Get("X-Client-Type"))
	userAgent := strings.TrimSpace(r.UserAgent())
	language := normalizeHeaderValue(firstNonEmpty(r.Header.Get("X-Language"), r.Header.Get("Accept-Language")))

	// 主流做法：优先绑定客户端稳定设备标识，避免将临时网络端口当作设备特征。
	if deviceID != "" {
		raw := strings.Join([]string{"did", deviceID, clientType, userAgent}, "|")
		return hashFingerprint(raw)
	}

	// 兼容旧客户端：当没有 X-Device-ID 时，退化为 IP(去端口)+UA+语言组合。
	clientIP := extractClientIP(r)
	raw := strings.Join([]string{"legacy", clientIP, clientType, userAgent, language}, "|")
	return hashFingerprint(raw)
}

// extractClientIP 提取客户端 IP，优先使用代理透传头，最后回退到 RemoteAddr（去掉端口）。
func extractClientIP(r *http.Request) string {
	if r == nil {
		return ""
	}

	xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if ip != "" {
				return ip
			}
		}
	}

	xri := strings.TrimSpace(r.Header.Get("X-Real-IP"))
	if xri != "" {
		return xri
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}

	return strings.TrimSpace(r.RemoteAddr)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func normalizeHeaderValue(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func hashFingerprint(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
