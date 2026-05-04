/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-29 19:52:08
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-05-03 21:47:32
 * @FilePath: /MLC_GO/internal/interfaces/middleware/hg_log_middle.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package HGMiddlewarePackage

import (
	"MLC_GO/internal/pkg/logHG"
	"hash/fnv"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const tag = "hg_access_log_interceptor"
const accessLogSampleRateEnv = "HG_ACCESS_LOG_SAMPLE_RATE"

var accessLogSampleRate = loadAccessLogSampleRate()

// loadAccessLogSampleRate 读取访问日志采样率，取值区间 [0,1]，默认全量记录。
func loadAccessLogSampleRate() float64 {
	raw := strings.TrimSpace(os.Getenv(accessLogSampleRateEnv))
	if raw == "" {
		return 1.0
	}

	rate, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 1.0
	}
	if rate < 0 {
		return 0
	}
	if rate > 1 {
		return 1
	}

	return rate
}

// shouldWriteAccessLog 按稳定哈希采样请求，避免高并发下日志成为瓶颈。
func shouldWriteAccessLog(r *http.Request) bool {
	if accessLogSampleRate >= 1.0 {
		return true
	}
	if accessLogSampleRate <= 0 {
		return false
	}

	key := strings.TrimSpace(r.Header.Get("X-Request-ID"))
	if key == "" {
		key = r.Method + "|" + r.URL.Path + "|" + r.RemoteAddr
	}

	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(key))
	bucket := float64(hasher.Sum32()%10000) / 10000.0

	return bucket < accessLogSampleRate
}

// AccessLogInterceptor 记录请求方法、路径和耗时，作为基础访问日志拦截器。
func AccessLogInterceptor(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		if !shouldWriteAccessLog(r) {
			return
		}

		logHG.DebugFInfo(
			`%s-日志拦截器： {"method":"%s", "path":"%s", "cost_ms":%d}`,
			tag,
			r.Method,
			r.URL.Path,
			time.Since(start).Milliseconds(),
		)
	})
}

// LoggerMiddleware 兼容旧方法名，内部转发到拦截器实现。
func LoggerMiddleware(next http.Handler) http.Handler {
	return AccessLogInterceptor(next)
}
