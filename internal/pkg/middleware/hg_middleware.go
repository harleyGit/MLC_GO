package HGMiddlewarePackage

import (
	"MLC_GO/internal/pkg/logHG"
	UtilsPackage "MLC_GO/internal/pkg/utils"
	HGResponsePakcage "MLC_GO/internal/response"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// Middleware 表示一个可组合的 HTTP 中间件。
type Middleware func(http.Handler) http.Handler

// Chain 在启动阶段把中间件链装配为最终 handler，避免请求期重复构建。
// 执行顺序：从右到左（最后一个中间件最先执行）。
func Chain(base http.Handler, middlewares ...Middleware) http.Handler {
	if base == nil {
		return nil
	}

	wrapped := base
	for i := len(middlewares) - 1; i >= 0; i-- {
		if middlewares[i] == nil {
			continue
		}
		wrapped = middlewares[i](wrapped)
	}

	return wrapped
}

// ChainInterceptors 兼容旧方法名。
func ChainInterceptors(base http.Handler, middlewares ...Middleware) http.Handler {
	return Chain(base, middlewares...)
}

// region 基础中间件

// RecoverMiddleware 统一拦截 panic，防止请求链路异常中断。
func RecoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				HGResponsePakcage.FailResult[any](w, r,
					HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InternalErrorCode, Message: "panic"},
				)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// RecoverInterceptor 兼容旧方法名。
func RecoverInterceptor(next http.Handler) http.Handler {
	return RecoverMiddleware(next)
}

// CORSMiddleware 统一处理跨域请求与预检响应。
func CORSMiddleware(next http.Handler) http.Handler {
	allowedOrigins := loadCORSAllowedOrigins()
	allowMethods := "GET, POST, PUT, DELETE, PATCH, OPTIONS"
	allowHeaders := strings.Join(corsAllowedHeaders, ", ")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin != "" {
			w.Header().Add("Vary", "Origin")
		}
		_, originAllowed := allowedOrigins[origin]
		if originAllowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		if r.Method == http.MethodOptions && origin != "" {
			if !originAllowed || !corsPreflightAllowed(r) {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.Header().Set("Access-Control-Allow-Methods", allowMethods)
			w.Header().Set("Access-Control-Allow-Headers", allowHeaders)
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func loadCORSAllowedOrigins() map[string]struct{} {
	raw := strings.TrimSpace(os.Getenv(corsAllowedOriginsEnv))
	origins := corsDefaultAllowedOrigins
	if raw != "" {
		origins = strings.Split(raw, ",")
	}
	allowed := make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			allowed[origin] = struct{}{}
		}
	}
	return allowed
}

func corsPreflightAllowed(r *http.Request) bool {
	method := strings.ToUpper(strings.TrimSpace(r.Header.Get("Access-Control-Request-Method")))
	if _, ok := corsAllowedMethods[method]; !ok {
		return false
	}
	for _, header := range strings.Split(r.Header.Get("Access-Control-Request-Headers"), ",") {
		header = strings.TrimSpace(header)
		if header != "" && !corsHeaderAllowed(header) {
			return false
		}
	}
	return true
}

func corsHeaderAllowed(header string) bool {
	for _, allowed := range corsAllowedHeaders {
		if strings.EqualFold(header, allowed) {
			return true
		}
	}
	return false
}

// CORSInterceptor 兼容旧方法名。
func CORSInterceptor(next http.Handler) http.Handler {
	return CORSMiddleware(next)
}

// JSONHeaderMiddleware 统一设置 JSON 响应头，保证客户端按 JSON 解析。
func JSONHeaderMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		next.ServeHTTP(w, r)
	})
}

// JSONHeaderInterceptor 兼容旧方法名。
func JSONHeaderInterceptor(next http.Handler) http.Handler {
	return JSONHeaderMiddleware(next)
}

// RequestIDMiddleware 注入请求链路追踪 ID，日志统一交给 AccessLogMiddleware 采样输出。
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tid := UtilsPackage.GenerateTID()
		ctx := UtilsPackage.InjectTID(r.Context(), tid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestTIDInterceptor 兼容旧方法名。
func RequestTIDInterceptor(next http.Handler) http.Handler {
	return RequestIDMiddleware(next)
}

// TIDMiddleware 兼容旧方法名。
func TIDMiddleware(next http.Handler) http.Handler {
	return RequestIDMiddleware(next)
}

// endregion

// region 访问日志中间件（带采样）

const accessLogTag = "access_log"
const accessLogSampleRateEnv = "HG_ACCESS_LOG_SAMPLE_RATE"

const corsAllowedOriginsEnv = "HG_CORS_ALLOWED_ORIGINS"

var corsDefaultAllowedOrigins = []string{
	"http://localhost:5173",
	"http://localhost:5174",
	"http://127.0.0.1:5173",
	"http://127.0.0.1:5174",
}

var corsAllowedMethods = map[string]struct{}{
	http.MethodGet:     {},
	http.MethodPost:    {},
	http.MethodPut:     {},
	http.MethodDelete:  {},
	http.MethodPatch:   {},
	http.MethodOptions: {},
}

var corsAllowedHeaders = []string{
	"Accept",
	"Authorization",
	"Content-Type",
	"X-API-Version",
	"X-Client-Type",
	"X-Client-Version",
	"X-Device-ID",
	"X-Language",
	"X-Request-ID",
	"X-Signature",
	"X-Timestamp",
}

var accessLogSampleRate = loadAccessLogSampleRate()

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

	bucket := float64(fnv32aString(key)%10000) / 10000.0

	return bucket < accessLogSampleRate
}

func fnv32aString(value string) uint32 {
	var hash uint32 = 2166136261
	for i := 0; i < len(value); i++ {
		hash ^= uint32(value[i])
		hash *= 16777619
	}
	return hash
}

// AccessLogMiddleware 记录请求方法、路径和耗时，作为基础访问日志中间件。
func AccessLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		if !shouldWriteAccessLog(r) {
			return
		}

		logHG.DebugFInfo(
			`%s: {"method":"%s", "path":"%s", "cost_ms":%d}`,
			accessLogTag,
			r.Method,
			r.URL.Path,
			time.Since(start).Milliseconds(),
		)
	})
}

// AccessLogInterceptor 兼容旧方法名。
func AccessLogInterceptor(next http.Handler) http.Handler {
	return AccessLogMiddleware(next)
}

// LoggerMiddleware 兼容旧方法名。
func LoggerMiddleware(next http.Handler) http.Handler {
	return AccessLogMiddleware(next)
}

// endregion
