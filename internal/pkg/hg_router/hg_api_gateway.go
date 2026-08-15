package HGRouterPackage

import (
	ConfigPackage "MLC_GO/internal/pkg/config"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	UtilsPackage "MLC_GO/internal/pkg/utils"
	HGResponsePakcage "MLC_GO/internal/response"
	"context"
	"errors"
	"math"
	"net"
	"net/http"
	"net/http/httputil"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

type hgGatewayRateEval interface {
	EvalInt64(context.Context, string, []string, ...any) (int64, error)
}

type hgGatewayRedisEval struct {
	redis *PersistenceRedisPackage.RedisService
}

func (e hgGatewayRedisEval) EvalInt64(ctx context.Context, script string, keys []string, args ...any) (int64, error) {
	return e.redis.Client().Eval(ctx, script, keys, args...).Int64()
}

type hgGatewayModule struct {
	name     string
	basePath string
	policy   ConfigPackage.HGAPIGatewayModulePolicy
	inFlight chan struct{}
	proxy    http.Handler
	metrics  *hgGatewayModuleMetrics
}

type hgGatewayModuleMetrics struct {
	requests      atomic.Uint64
	rateLimited   atomic.Uint64
	overloaded    atomic.Uint64
	redisFailures atomic.Uint64
}

// HGAPIGateway 是业务 HTTP 的统一入口层，负责模块识别、安全响应头和跨实例粗粒度限流。
// JWT、签名、权限和业务限流继续由现有模块中间件处理，避免重复解析请求或改变外部 API 语义。
type HGAPIGateway struct {
	enabled        bool
	eval           hgGatewayRateEval
	maxURLBytes    int
	versions       map[string]struct{}
	trustedProxies []netip.Prefix
	modules        []hgGatewayModule
	now            func() time.Time
}

var hgGatewayModulePaths = []struct {
	name     string
	basePath string
}{
	{"auth", AuthModuleBasePath},
	{"profile", UserProfileModuleBasePath},
	{"video_upload", VideoUploadModuleBasePath},
	{"bilibili", BilibiliModuleBasePath},
	{"video_recommend", VideoRecommendModuleBasePath},
	{"video_interaction", VideoInteractionModuleBasePath},
	{"video_comment", VideoCommentModuleBasePath},
	{"video_danmaku", VideoDanmakuModuleBasePath},
	{"ops", OpsModuleBasePath},
}

// NewHGAPIGateway 使用应用共享 Redis 连接池构造网关；启用时 Redis 不可用会直接拒绝启动。
func NewHGAPIGateway(redis *PersistenceRedisPackage.RedisService, config ConfigPackage.HGAPIGatewayConfig) (*HGAPIGateway, error) {
	if !config.Enabled {
		return &HGAPIGateway{enabled: false}, nil
	}
	if redis == nil || redis.Client() == nil {
		return nil, errors.New("API Gateway Redis 不可用")
	}
	return newHGAPIGateway(hgGatewayRedisEval{redis: redis}, config)
}

func newHGAPIGateway(eval hgGatewayRateEval, config ConfigPackage.HGAPIGatewayConfig) (*HGAPIGateway, error) {
	if !config.Enabled {
		return &HGAPIGateway{enabled: false}, nil
	}
	if eval == nil {
		return nil, errors.New("API Gateway rate eval 不能为空")
	}
	gateway := &HGAPIGateway{
		enabled:        true,
		eval:           eval,
		maxURLBytes:    config.MaxURLBytes,
		versions:       config.SupportedVersions,
		trustedProxies: append([]netip.Prefix(nil), config.TrustedProxyCIDRs...),
		modules:        make([]hgGatewayModule, 0, len(hgGatewayModulePaths)),
		now:            time.Now,
	}
	for _, module := range hgGatewayModulePaths {
		policy, ok := config.Modules[module.name]
		if !ok || policy.Capacity < 1 || policy.RefillPerSecond <= 0 || policy.MaxBodyBytes < 1 || policy.MaxInFlight < 1 {
			return nil, errors.New("API Gateway 模块限流配置不完整")
		}
		gatewayModule := hgGatewayModule{
			name: module.name, basePath: module.basePath, policy: policy,
			inFlight: make(chan struct{}, policy.MaxInFlight), metrics: &hgGatewayModuleMetrics{},
		}
		if policy.UpstreamURL != "" {
			target, err := url.Parse(policy.UpstreamURL)
			if err != nil {
				return nil, errors.New("API Gateway 模块 upstream_url 无效")
			}
			gatewayModule.proxy = newHGModuleReverseProxy(target)
		}
		gateway.modules = append(gateway.modules, gatewayModule)
	}
	return gateway, nil
}

// Middleware 在根路由匹配和 StripPrefix 之前执行，因此 React 公共 URL 与模块内签名路径保持不变。
func (g *HGAPIGateway) Middleware(next http.Handler) http.Handler {
	if g == nil || !g.enabled {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.setSecurityHeaders(w, r)
		if len(r.URL.RequestURI()) > g.maxURLBytes {
			g.writeFailure(w, r, http.StatusRequestURITooLong, HGResponsePakcage.RequestURI)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") && !g.acceptVersion(r) {
			g.writeFailure(w, r, http.StatusBadRequest, HGResponsePakcage.RequestHeader)
			return
		}
		module, ok := g.matchModule(r.URL.Path)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		module.metrics.requests.Add(1)
		if r.ContentLength > module.policy.MaxBodyBytes {
			g.writeFailure(w, r, http.StatusRequestEntityTooLarge, HGResponsePakcage.RequestTooLarge)
			return
		}
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, module.policy.MaxBodyBytes)
		}
		select {
		case module.inFlight <- struct{}{}:
			defer func() { <-module.inFlight }()
		default:
			module.metrics.overloaded.Add(1)
			w.Header().Set("Retry-After", "1")
			g.writeFailure(w, r, http.StatusServiceUnavailable, HGResponsePakcage.ServiceUnavailable)
			return
		}

		sourceIP := g.sourceIP(r)
		ttlSeconds := int64(math.Ceil(float64(module.policy.Capacity)/module.policy.RefillPerSecond)) * 2
		if ttlSeconds < 1 {
			ttlSeconds = 1
		}
		allowed, err := g.eval.EvalInt64(
			r.Context(),
			PersistenceRedisPackage.TokenBucketRateLimitLuaScript,
			[]string{PersistenceRedisPackage.GetAPIGatewayIPRateKey(module.name, sourceIP)},
			module.policy.Capacity,
			module.policy.RefillPerSecond,
			g.now().UnixMilli(),
			1,
			ttlSeconds,
		)
		if err != nil {
			module.metrics.redisFailures.Add(1)
			g.writeFailure(w, r, http.StatusServiceUnavailable, HGResponsePakcage.ServiceUnavailable)
			return
		}
		if allowed != 1 {
			module.metrics.rateLimited.Add(1)
			retryAfter := int(math.Ceil(1 / module.policy.RefillPerSecond))
			if retryAfter < 1 {
				retryAfter = 1
			}
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			g.writeFailure(w, r, http.StatusTooManyRequests, HGResponsePakcage.RateLimit)
			return
		}
		if module.proxy != nil {
			r.Header.Set("X-Forwarded-For", sourceIP)
			r.Header.Set("X-Real-IP", sourceIP)
			module.proxy.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func newHGModuleReverseProxy(target *url.URL) http.Handler {
	proxy := &httputil.ReverseProxy{}
	proxy.Rewrite = func(request *httputil.ProxyRequest) {
		request.SetURL(target)
		sourceIP := request.Out.Header.Get("X-Real-IP")
		request.Out.Header.Set("X-Forwarded-For", sourceIP)
		request.Out.Header.Set("X-Real-IP", sourceIP)
		request.Out.Header.Del("Forwarded")
	}
	proxy.Transport = &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          4096,
		MaxIdleConnsPerHost:   1024,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, _ error) {
		tid := UtilsPackage.GetTID(r.Context())
		if tid == "" {
			tid = UtilsPackage.GenerateTID()
			r = r.WithContext(UtilsPackage.InjectTID(r.Context(), tid))
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("X-Request-ID", tid)
		w.WriteHeader(http.StatusBadGateway)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.ServiceUnavailable)
	}
	return proxy
}

func (g *HGAPIGateway) matchModule(path string) (hgGatewayModule, bool) {
	for _, module := range g.modules {
		if path == module.basePath || strings.HasPrefix(path, module.basePath+"/") {
			return module, true
		}
	}
	return hgGatewayModule{}, false
}

func (g *HGAPIGateway) setSecurityHeaders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
	w.Header().Set("X-Frame-Options", "DENY")
	if strings.HasPrefix(r.URL.Path, AuthModuleBasePath+"/") || strings.HasPrefix(r.URL.Path, OpsModuleBasePath+"/") {
		w.Header().Set("Cache-Control", "no-store")
	}
}

func (g *HGAPIGateway) acceptVersion(r *http.Request) bool {
	const apiPrefix = "/api/"
	if !strings.HasPrefix(r.URL.Path, apiPrefix) {
		return false
	}
	remainder := strings.TrimPrefix(r.URL.Path, apiPrefix)
	separator := strings.IndexByte(remainder, '/')
	if separator < 1 {
		return false
	}
	pathVersion := remainder[:separator]
	if _, ok := g.versions[pathVersion]; !ok {
		return false
	}
	headerVersion := strings.TrimSpace(r.Header.Get("X-API-Version"))
	return headerVersion == "" || headerVersion == pathVersion
}

func (g *HGAPIGateway) writeFailure(w http.ResponseWriter, r *http.Request, status int, result HGResponsePakcage.HGErrorResult) {
	tid := UtilsPackage.GetTID(r.Context())
	if tid == "" {
		tid = UtilsPackage.GenerateTID()
		r = r.WithContext(UtilsPackage.InjectTID(r.Context(), tid))
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Request-ID", tid)
	w.WriteHeader(status)
	HGResponsePakcage.FailResult[string](w, r, result)
}

func (g *HGAPIGateway) sourceIP(r *http.Request) string {
	direct := hgParseIP(r.RemoteAddr)
	if !g.isTrustedProxy(direct) {
		return hgIPString(direct)
	}

	forwarded := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for index := len(forwarded) - 1; index >= 0; index-- {
		candidate := hgParseIP(strings.TrimSpace(forwarded[index]))
		if !candidate.IsValid() {
			continue
		}
		if !g.isTrustedProxy(candidate) {
			return candidate.String()
		}
		direct = candidate
	}
	if realIP := hgParseIP(r.Header.Get("X-Real-IP")); realIP.IsValid() {
		return realIP.String()
	}
	return hgIPString(direct)
}

func (g *HGAPIGateway) isTrustedProxy(ip netip.Addr) bool {
	if !ip.IsValid() {
		return false
	}
	for _, prefix := range g.trustedProxies {
		if prefix.Contains(ip) {
			return true
		}
	}
	return false
}

func hgParseIP(value string) netip.Addr {
	value = strings.TrimSpace(value)
	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	}
	ip, _ := netip.ParseAddr(strings.Trim(value, "[]"))
	return ip.Unmap()
}

func hgIPString(ip netip.Addr) string {
	if ip.IsValid() {
		return ip.String()
	}
	return "unknown"
}
