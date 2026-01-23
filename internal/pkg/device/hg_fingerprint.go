/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-23 13:49:26
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-23 13:52:38
 * @FilePath: /MLC_GO/internal/pkg/device/hg_fingerprint.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package PkGDevicePackage

import (
	"crypto/sha1"
	"fmt"
	"net/http"
)

/* 设备指纹 + 风控升级 */
// TODO：Redis中的风控Key： risk:device:{fingerprint} -> count /TTL
func Fingerprint(r *http.Request) string {
	raw := fmt.Sprintf("%s|%s%s",
		r.RemoteAddr,
		r.UserAgent(),
		r.Header.Get("Accept-Language"),
	)
	h := sha1.Sum([]byte(raw))
	return fmt.Sprintf("%x", h)
}
