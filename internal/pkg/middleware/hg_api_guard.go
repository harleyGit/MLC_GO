package HGMiddlewarePackage

import (
	UserServicePackage "MLC_GO/internal/modules/user/service"
	"MLC_GO/internal/pkg/logHG"
	UtilsPackage "MLC_GO/internal/pkg/utils"
	HGResponsePakcage "MLC_GO/internal/response"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"hash"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const apiGuardTag = "api_guard"

const (
	apiGuardMaxSignedBodyBytes = 1 << 20
	methodGet                  = 1 << iota
	methodPost
	methodPut
	methodDelete
	methodPatch
	methodOptions
	methodHead
)

// region 数据结构定义

// APIRule 定义单个 API 路由规则。
type APIRule struct {
	Path        string          // 路由路径
	Methods     map[string]bool // 允许的 HTTP 方法
	NeedAuth    bool            // 是否需要认证
	Permissions []string        // 需要的权限列表
	Version     string          // API 版本
}

// HGAPIRule 兼容旧类型名。
type HGAPIRule = APIRule

type ctxKey string

const (
	CtxUserID         ctxKey = "userID"
	CtxDeviceID       ctxKey = "deviceID"
	defaultAPIVersion        = "v1"
)

// APIGuard 存储 API 规则，支持版本化路由。
type APIGuard struct {
	rulesByVersion map[string]map[string]compiledAPIRule
	legacyRules    map[string]compiledAPIRule
}

// compiledAPIRule 是启动期预编译后的 API 规则，避免请求期反复遍历/查 map。
type compiledAPIRule struct {
	APIRule
	methodMask uint16
}

// endregion

// region 权限配置

var rolePermissions = map[string]map[string]bool{
	"admin": {
		"user:update": true,
		"user:view":   true,
	},
	"user": {
		"user:view": true,
	},
}

// endregion

// region 构造函数

// NewAPIGuard 创建 API 规则守卫。
func NewAPIGuard(rules []APIRule) *APIGuard {
	guard := &APIGuard{
		rulesByVersion: make(map[string]map[string]compiledAPIRule),
		legacyRules:    make(map[string]compiledAPIRule),
	}

	for _, r := range rules {
		version := strings.TrimSpace(r.Version)
		if version == "" {
			version = defaultAPIVersion
		}

		compiled := compiledAPIRule{
			APIRule:    r,
			methodMask: compileMethodMask(r.Methods),
		}

		if _, ok := guard.rulesByVersion[version]; !ok {
			guard.rulesByVersion[version] = make(map[string]compiledAPIRule)
		}

		guard.rulesByVersion[version][r.Path] = compiled
		guard.legacyRules[r.Path] = compiled
	}

	return guard
}

// endregion

// region 中间件入口

// APIGuardMiddleware 快速构建 API Guard 中间件。
func APIGuardMiddleware(rules []APIRule) Middleware {
	return NewAPIGuard(rules).Middleware()
}

// APIGuardInterceptor 兼容旧方法名。
func APIGuardInterceptor(rules []APIRule) Middleware {
	return APIGuardMiddleware(rules)
}

// Middleware 执行 Method/Header/Auth/Permission 统一拦截。
func (g *APIGuard) Middleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			version := strings.TrimSpace(r.Header.Get("X-API-Version"))
			if version == "" {
				version = defaultAPIVersion
			}

			rule, ok := g.lookupRule(version, r.URL.Path)
			if !ok {
				http.NotFound(w, r)
				HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.RequestURI.Code, Message: "interface not found"})
				logHG.ErrFInfo(`%s: 未找到接口规则，version="%s", path="%s"`, apiGuardTag, version, r.URL.Path)
				return
			}

			if !rule.allowMethod(r.Method) {
				w.WriteHeader(http.StatusMethodNotAllowed)
				HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.RequestMethod.Code, Message: "method not allowed"})
				logHG.ErrFInfo(`%s: Method校验失败，version="%s", path="%s"`, apiGuardTag, version, r.URL.Path)
				return
			}

			ctx := g.checkoutHeader(w, r, rule.NeedAuth)
			if ctx == nil {
				HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.RequestHeader.Code, Message: "header valid fail"})
				logHG.ErrFInfo(`%s: Header校验失败，version="%s", path="%s"`, apiGuardTag, version, r.URL.Path)
				return
			}

			if len(rule.Permissions) > 0 {
				role := r.Header.Get("X-Role")
				if role == "" {
					role = "user"
				}
				if !HasPermission(role, rule.Permissions) {
					w.WriteHeader(http.StatusForbidden)
					HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.Forbidden.Code, Message: "权限校验失败"})
					return
				}
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// MethodGuardMiddlewareV3 兼容旧方法名。
func (g *APIGuard) MethodGuardMiddlewareV3(next http.Handler) http.Handler {
	return g.Middleware()(next)
}

// endregion

// region 内部方法

func (g *APIGuard) lookupRule(version string, path string) (compiledAPIRule, bool) {
	routesByPath, ok := g.rulesByVersion[version]
	if !ok {
		if version != defaultAPIVersion {
			if fallbackRoutes, fallbackOK := g.rulesByVersion[defaultAPIVersion]; fallbackOK {
				rule, ruleOK := fallbackRoutes[path]
				return rule, ruleOK
			}
		}
		return compiledAPIRule{}, false
	}

	rule, ok := routesByPath[path]
	return rule, ok
}

func (g *APIGuard) checkoutHeader(w http.ResponseWriter, r *http.Request, needAuth bool) context.Context {
	token := r.Header.Get("Authorization")
	contentType := r.Header.Get("Content-Type")
	deviceID := r.Header.Get("X-Device-ID")
	clientType := r.Header.Get("X-Client-Type")
	clientVersion := r.Header.Get("X-Client-Version")
	version := r.Header.Get("X-API-Version")
	language := r.Header.Get("X-Language")
	requestID := r.Header.Get("X-Request-ID")
	timestamp := r.Header.Get("X-Timestamp")
	signature := r.Header.Get("X-Signature")

	if UtilsPackage.IsEmpty(contentType) ||
		UtilsPackage.IsEmpty(deviceID) ||
		UtilsPackage.IsEmpty(clientType) ||
		UtilsPackage.IsEmpty(clientVersion) ||
		UtilsPackage.IsEmpty(version) ||
		UtilsPackage.IsEmpty(language) ||
		UtilsPackage.IsEmpty(requestID) ||
		UtilsPackage.IsEmpty(timestamp) ||
		UtilsPackage.IsEmpty(signature) {

		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.RequestHeader.Code, Message: HGResponsePakcage.RequestHeaderFailDesc})
		return nil
	}

	if needAuth && UtilsPackage.IsEmpty(token) {
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePakcage.FailTokenInvalid(w, r, "Authorization不能为空")
		return nil
	}

	if err := verifyTimestamp(timestamp); err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePakcage.FailTokenInvalid(w, r, "timestamp无效或已过期")
		return nil
	}

	// 对于 multipart/form-data 或二进制数据请求，使用空字符串作为 body 签名
	// 前端签名时也使用空字符串，保证前后端一致
	var body []byte
	isBinaryUpload := strings.HasPrefix(contentType, "multipart/form-data") ||
		strings.HasPrefix(contentType, "application/octet-stream")
	if !isBinaryUpload {
		var err error
		body, err = readAndRestoreBody(r, apiGuardMaxSignedBodyBytes)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.RequestHeader.Code, Message: "请求体读取失败"})
			return nil
		}
	}

	if err := verifySignature(r, body, signature, timestamp, requestID, deviceID, clientType, clientVersion, version, language, token); err != nil {
		logHG.ErrFInfo("signature无效，请求可能被篡改,错误码：%d, 业务错误码：%d", http.StatusUnauthorized, HGResponsePakcage.Unauthorized.Code)
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePakcage.FailTokenInvalid(w, r, "signature无效")
		return nil
	}

	ctx := r.Context()
	ctx = context.WithValue(ctx, CtxDeviceID, deviceID)

	return ctx
}

// endregion

// region 签名验证

func verifyTimestamp(timestamp string) error {
	ts, err := strconv.ParseInt(strings.TrimSpace(timestamp), 10, 64)
	if err != nil {
		return err
	}

	now := time.Now().Unix()
	if ts <= 0 || absInt64(now-ts) > int64(5*time.Minute/time.Second) {
		return errors.New("timestamp expired")
	}

	return nil
}

func verifySignature(
	r *http.Request,
	body []byte,
	signature string,
	timestamp string,
	requestID string,
	deviceID string,
	clientType string,
	clientVersion string,
	version string,
	language string,
	token string,
) error {
	signature = strings.TrimSpace(signature)
	if signature == "" {
		return errors.New("empty signature")
	}

	signature = strings.TrimPrefix(signature, "sha256=")
	signatureBytes, err := hex.DecodeString(signature)
	if err != nil {
		return err
	}

	expected := buildRequestSignature(
		r, body, timestamp, requestID, deviceID,
		clientType, clientVersion, version, language, token,
	)

	if !hmac.Equal(signatureBytes, expected) {
		return errors.New("signature mismatch")
	}

	return nil
}

func buildRequestSignature(
	r *http.Request,
	body []byte,
	timestamp string,
	requestID string,
	deviceID string,
	clientType string,
	clientVersion string,
	version string,
	language string,
	token string,
) []byte {
	mac := hmac.New(sha256.New, UserServicePackage.Secret)
	writeSignaturePart(mac, r.Method)
	writeSignaturePart(mac, r.URL.Path)
	writeSignaturePart(mac, timestamp)
	writeSignaturePart(mac, requestID)
	writeSignaturePart(mac, deviceID)
	writeSignaturePart(mac, clientType)
	writeSignaturePart(mac, clientVersion)
	writeSignaturePart(mac, version)
	writeSignaturePart(mac, language)
	writeSignaturePart(mac, hex.EncodeToString(hashBody(body)))
	_, _ = mac.Write([]byte(token))

	return mac.Sum(nil)
}

func readAndRestoreBody(r *http.Request, limit int64) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	if limit > 0 && r.ContentLength > limit {
		return nil, errors.New("request body too large")
	}

	reader := io.Reader(r.Body)
	if limit > 0 {
		reader = io.LimitReader(r.Body, limit+1)
	}
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if limit > 0 && int64(len(body)) > limit {
		return nil, errors.New("request body too large")
	}

	r.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

func hashBody(body []byte) []byte {
	bodyHash := sha256.Sum256(body)
	return bodyHash[:]
}

func writeSignaturePart(mac hash.Hash, value string) {
	_, _ = mac.Write([]byte(value))
	_, _ = mac.Write([]byte("\n"))
}

func absInt64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// endregion

// region 权限判断

// HasPermission 判断角色是否拥有所需权限。
func HasPermission(role string, perms []string) bool {
	rolePerms := rolePermissions[role]
	if rolePerms == nil {
		return false
	}
	for _, p := range perms {
		if rolePerms[p] {
			return true
		}
	}
	return false
}

// endregion

// region 兼容旧方法

func MethodGuardMiddlewareV2(rules []APIRule) func(http.Handler) http.Handler {
	ruleMap := make(map[string]compiledAPIRule)
	for _, r := range rules {
		ruleMap[r.Path] = compiledAPIRule{
			APIRule:    r,
			methodMask: compileMethodMask(r.Methods),
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rule, ok := ruleMap[r.URL.Path]
			if !ok {
				http.NotFound(w, r)
				return
			}

			if !rule.allowMethod(r.Method) {
				w.WriteHeader(http.StatusMethodNotAllowed)
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.RequestMethod.Code, Message: "method not allowed"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (g *APIGuard) MethodGuardMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rule, ok := g.legacyRules[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}

		if !rule.allowMethod(r.Method) {
			w.WriteHeader(http.StatusMethodNotAllowed)
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.RequestMethod.Code, Message: "method not allowed"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// endregion

func compileMethodMask(methods map[string]bool) uint16 {
	var mask uint16
	for method, allowed := range methods {
		if !allowed {
			continue
		}
		mask |= methodBit(method)
	}
	return mask
}

func (r compiledAPIRule) allowMethod(method string) bool {
	return r.methodMask&methodBit(method) != 0
}

func methodBit(method string) uint16 {
	switch method {
	case http.MethodGet:
		return methodGet
	case http.MethodPost:
		return methodPost
	case http.MethodPut:
		return methodPut
	case http.MethodDelete:
		return methodDelete
	case http.MethodPatch:
		return methodPatch
	case http.MethodOptions:
		return methodOptions
	case http.MethodHead:
		return methodHead
	default:
		return 0
	}
}
