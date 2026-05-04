/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-02-01 12:30:27
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-05-04 16:00:49
 * @FilePath: /MLC_GO/internal/interfaces/middleware/hg_method_guard_middle.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
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
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

/*
Permissions权限的几种配置

	var rolePermissions = map[string]map[string]bool{
	    "admin": {
	        "user:update": true,    // admin 角色有 user:update 权限
	        "user:view":   true,    // admin 角色有 user:view 权限
	    },
	    "user": {
	        "user:view": true,      // user 角色只有 user:view 权限
	    },
	}
*/
type HGAPIRule struct {
	Path        string          // 路由路径
	Methods     map[string]bool // 允许的 HTTP 方法
	NeedAuth    bool            // 是否需要认证
	Permissions []string        // 需要的权限列表
	Version     string          // API 版本
}

type ctxKey string

const (
	CtxUserID         ctxKey = "userID"
	CtxDeviceID       ctxKey = "deviceID"
	defaultAPIVersion        = "v1"
)

/*
	 HGAPIGuard存放数据结构：

				guard.rulesByVersion:
			{
			    "v1": {
			        "/info":  {Path: "/info", NeedAuth: true, ...},
			        "/login": {Path: "/login", NeedAuth: false, ...},
			    },
			    "v2": {
			        "/info":  {Path: "/info", NeedAuth: true, ...},
			    }
			}

			guard.legacyRules:
			{
			    "/info":  {Path: "/info", NeedAuth: true, ...},
			    "/login": {Path: "/login", NeedAuth: false, ...},
			}
*/
type HGAPIGuard struct {
	rulesByVersion map[string]map[string]HGAPIRule
	legacyRules    map[string]HGAPIRule
}

var rolePermissions = map[string]map[string]bool{
	"admin": { //管理员，可以查看和更新
		"user:update": true,
		"user:view":   true,
	},
	"user": {// 普通用户，只能查看
		"user:view": true,
	},
}

func NewAPIGuard(rules []HGAPIRule) *HGAPIGuard {
	guard := &HGAPIGuard{
		rulesByVersion: make(map[string]map[string]HGAPIRule),
		legacyRules:    make(map[string]HGAPIRule),
	}

	for _, r := range rules {
		version := strings.TrimSpace(r.Version)
		if version == "" {
			version = defaultAPIVersion // 默认 "v1"
		}

		// 初始化内层 map（如果不存在）
		if _, ok := guard.rulesByVersion[version]; !ok {
			guard.rulesByVersion[version] = make(map[string]HGAPIRule)
		}

		guard.rulesByVersion[version][r.Path] = r
		guard.legacyRules[r.Path] = r
	}

	return guard
}

// APIGuardInterceptor 快速构建 API Guard 拦截器，供路由链直接复用。
func APIGuardInterceptor(rules []HGAPIRule) HGHTTPInterceptor {
	return NewAPIGuard(rules).Interceptor()
}

// Interceptor 执行 Method/Header/Auth/Permission 统一拦截。
func (g *HGAPIGuard) Interceptor() HGHTTPInterceptor {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 2.2 获取版本
			version := strings.TrimSpace(r.Header.Get("X-API-Version"))
			if version == "" {
				version = defaultAPIVersion
			}

			// 2.3 获取路由规则，r.URL.Path 是 StripPrefix 之后的路径【接口路径】
			rule, ok := g.lookupRule(version, r.URL.Path)
			if !ok {
				http.NotFound(w, r)
				HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.NotFoundCode, "interface not found")
				logHG.ErrFInfo(`%s-APIGuard拦截器： 未找到接口规则，version="%s", path="%s"`, tag, version, r.URL.Path)
				return
			}

			// 1️⃣ Method 校验
			if !rule.Methods[r.Method] {
				w.WriteHeader(http.StatusMethodNotAllowed)
				HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.MethodNotAllowCode, "method not allowed")
				logHG.ErrFInfo(`%s-Method检验拦截器： Method校验失败，version="%s", path="%s"`, tag, version, r.URL.Path)
				return
			}

			// 2.1 header检验
			ctx := g.checkoutHeader(w, r, rule.NeedAuth)
			if ctx == nil {
				HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.RuequestHeaderNotOk, "heaer vaild fail")
				logHG.ErrFInfo(`%s-header检验拦截器： Method校验失败，version="%s", path="%s"`, tag, version, r.URL.Path)
				return
			}

			// 3️⃣ 权限校验（可选）
			if len(rule.Permissions) > 0 {
				role := r.Header.Get("X-Role")  // 从前端 header 获取角色
				if role == "" {
					role = "user"  // 默认为普通用户
				}
				if !HasPermission(role, rule.Permissions) {
					w.WriteHeader(http.StatusForbidden)
					HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.ForbiddenCode, "权限校验失败❌")
					return
				}
			}
			// 权限校验已移至 PermissionInterceptor，在 JWT 中间件之后执行
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// MethodGuardMiddlewareV3 兼容旧方法名，内部使用拦截器实现。
func (g *HGAPIGuard) MethodGuardMiddlewareV3(next http.Handler) http.Handler {
	return g.Interceptor()(next)
}

/*
 * 获取接口规则
 */
func (g *HGAPIGuard) lookupRule(version string, path string) (HGAPIRule, bool) {
	// comma-ok模式：map、channel、类型判断均可以这么做
	routesByPath, ok := g.rulesByVersion[version]
	if !ok {
		// 兼容兜底：客户端传错版本头时，尝试回退到默认版本路由规则。
		if version != defaultAPIVersion {
			if fallbackRoutes, fallbackOK := g.rulesByVersion[defaultAPIVersion]; fallbackOK {
				rule, ruleOK := fallbackRoutes[path]
				return rule, ruleOK
			}
		}
		return HGAPIRule{}, false
	}

	rule, ok := routesByPath[path]
	return rule, ok
}

// checkoutHeader 统一校验接口请求头、JWT 和签名，并把通过校验的用户上下文写回请求链路。
// 这里同时负责时间戳、防篡改签名和设备信息校验，避免请求头合法但请求内容已被替换。
func (g *HGAPIGuard) checkoutHeader(w http.ResponseWriter, r *http.Request, needAuth bool) context.Context {

	// ===== 1. Header 校验 =====
	token := r.Header.Get("Authorization")            // Authorization: Bearer <access_token>
	conentType := r.Header.Get("Content-Type")        // Content-Type: application/json
	deviceID := r.Header.Get("X-Device-ID")           // X-Device-ID: a1b2c3d4-e5f6-7890
	clientType := r.Header.Get("X-Client-Type")       // X-Client-Type: ios / android / web
	clientVersion := r.Header.Get("X-Client-Version") // 设备版本号：1.0.0
	version := r.Header.Get("X-API-Version")          // X-Client-Version: 2.1.0
	languange := r.Header.Get("X-Language")           // Accept-Language: zh-CN,zh;q=0.9,en;q=0.8 【想要 简体中文（中国） 的内容；如果没有，其他中文也行；如果连中文都没有，那就给我 英文 吧； q 是 “quality factor”（质量因子）的缩写，取值范围是 0.0 ~ 1.0，数值越大，优先级越高】
	requestid := r.Header.Get("X-Request-ID")         // 【添加请求 ID（用于日志追踪）：】X-Request-ID: abc123def456
	timestamp := r.Header.Get("X-Timestamp")          // X-Timestamp: 1700000000
	signature := r.Header.Get("X-Signature")          // TODO：后端验证签名，防止中间人伪造请求，请求体 + 时间戳 + 密钥进行 HMAC 签名，放入 Header：X-Signature: sha256=8f42a1b3c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2

	if UtilsPackage.IsEmpty(conentType) ||
		UtilsPackage.IsEmpty(deviceID) ||
		UtilsPackage.IsEmpty(clientType) ||
		UtilsPackage.IsEmpty(clientVersion) ||
		UtilsPackage.IsEmpty(version) ||
		UtilsPackage.IsEmpty(languange) ||
		UtilsPackage.IsEmpty(requestid) ||
		UtilsPackage.IsEmpty(timestamp) ||
		UtilsPackage.IsEmpty(signature) {

		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.RuequestHeaderNotOk, HGResponsePakcage.RequestHeaderFailDesc)
		return nil
	}

	// needAuth 为 true 时，Authorization 必须存在，避免受保护接口在空 token 场景下仅靠签名通过。
	if needAuth && UtilsPackage.IsEmpty(token) {
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.UnauthorizedCode, "Authorization不能为空")
		return nil
	}

	if err := verifyTimestamp(timestamp); err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.TokenInvalidCode, "timestamp无效或已过期")
		return nil
	}

	body, err := readAndRestoreBody(r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.RuequestHeaderNotOk, "请求体读取失败")
		return nil
	}

	if err := verifySignature(r, body, signature, timestamp, requestid, deviceID, clientType, clientVersion, version, languange, token); err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.TokenInvalidCode, "signature无效，请求可能被篡改")
		return nil
	}

	// ===== 2. 写入 Context =====
	// 鉴权 token 的解析交给下游 JWT 拦截器，避免 APIGuard 与业务鉴权重复计算。
	ctx := r.Context()
	ctx = context.WithValue(ctx, CtxDeviceID, deviceID)

	return ctx
}

// 验证传入的时间戳是否在允许的时间范围内，用于拦截过期或重放请求。
// 当前按 5 分钟容忍窗口处理，兼顾客户端时钟偏差和安全性。
func verifyTimestamp(timestamp string) error {
	// strings.TrimSpace 去掉首尾空格
	// strconv.ParseInt(..., 10, 64) 将字符串按十进制转换为 int64 类型。
	ts, err := strconv.ParseInt(strings.TrimSpace(timestamp), 10, 64)
	if err != nil {
		return err
	}

	now := time.Now().Unix()
	// 校验时间戳是否有效
	// absInt64(now-ts) → 当前时间与传入时间戳的绝对差值
	// int64(5*time.Minute/time.Second) → 计算允许的时间误差（这里是 5 分钟 → 5*60=300 秒）
	if ts <= 0 || absInt64(now-ts) > int64(5*time.Minute/time.Second) {
		return errors.New("timestamp expired")
	}

	return nil
}

// verifySignature 按约定的请求签名串重新计算 HMAC，判断关键头和请求体是否被篡改。
// 这里只接受当前约定的 sha256 签名格式，签名不一致时直接拦截请求。
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
	// 把十六进制字符串转换成 字节数组 ([]byte)
	signatureBytes, err := hex.DecodeString(signature)
	if err != nil {
		return err
	}

	expected := buildRequestSignature(
		r,
		body,
		timestamp,
		requestID,
		deviceID,
		clientType,
		clientVersion,
		version,
		language,
		token,
	)

	// 校验签名,hmac.Equal → 安全比较两个字节数组，防止 时间侧信道攻击（直接用 bytes.Equal 会有泄露风险
	if !hmac.Equal(signatureBytes, expected) {
		return errors.New("signature mismatch")
	}

	return nil
}

/* 生成 服务器端计算的签名，用来和请求中提供的签名比对 */
// buildRequestSignature 把方法、路径、时间戳、设备信息和请求体摘要拼成稳定签名串。
// 这样可以在不直接暴露原始 body 的前提下，对请求关键内容做完整性校验。
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
	// 对请求体 (body) 做 SHA256 哈希，得到固定长度的摘要
	// 这样签名绑定的是请求内容，而不是明文 body，安全性更高。
	bodyHash := sha256.Sum256(body)
	// 拼接签名原文（payload）
	payload := strings.Join([]string{
		r.Method,
		r.URL.Path,
		timestamp,
		requestID,
		deviceID,
		clientType,
		clientVersion,
		version,
		language,
		hex.EncodeToString(bodyHash[:]),
		token,
	}, "\n")

	// 使用 HMAC-SHA256，密钥为 UserServicePackage.Secret。
	mac := hmac.New(sha256.New, UserServicePackage.Secret)
	// 将拼接好的原文写入 HMAC
	mac.Write([]byte(payload))

	// 返回最终签名的字节数组 ([]byte)。
	return mac.Sum(nil)
}

// readAndRestoreBody 读取请求体后重新放回 r.Body，避免后续 handler 再读 body 时拿到空数据。
// 该方法只做透明恢复，不改变请求体内容本身。
func readAndRestoreBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}

	// io.ReadAll(r.Body) 会把流里所有的数据读出来，返回一个字节切片 []byte
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	// 因为上一步已经把原来的 r.Body 读光了，后续如果中间件或者处理函数还想再读取 r.Body 会失败
	// bytes.NewReader(body) 把读出来的 body 数据重新变成一个 io.Reader。
	// io.NopCloser(...) 把这个 io.Reader 包装成一个 io.ReadCloser（实现了 Close() 方法），这样就可以赋值回 r.Body，模拟原来的请求体
	r.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

// absInt64 返回 int64 的绝对值，用于时间戳窗口校验时避免正负差值分支重复。
func absInt64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

/* 权限判断 */
func HasPermission(role string, perms []string) bool {

	rolePerms := rolePermissions[role]
	for _, p := range perms {
		if rolePerms[p] {
			return true
		}
	}
	return false
}

func MethodGuardMiddlewareV2(rules []HGAPIRule) func(http.Handler) http.Handler {

	ruleMap := make(map[string]HGAPIRule)
	for _, r := range rules {
		ruleMap[r.Path] = r
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			// r.URL.Path 是 StripPrefix 之后的路径
			rule, ok := ruleMap[r.URL.Path]
			if !ok {
				// 未注册接口，直接 404
				http.NotFound(w, r)
				return
			}

			if !rule.Methods[r.Method] { //handler / service 完全不会执行
				// 🔥 Method 不允许，直接中断
				w.WriteHeader(http.StatusMethodNotAllowed)
				HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.MethodNotFoundCode, "method not allowed")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (m *HGAPIGuard) MethodGuardMiddleware(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		rule, ok := m.legacyRules[r.URL.Path]
		if !ok {
			// 未注册接口，直接 404
			http.NotFound(w, r)
			return
		}

		if !rule.Methods[r.Method] { //handler / service 完全不会执行
			// 🔥 Method 不允许，直接中断
			w.WriteHeader(http.StatusMethodNotAllowed)
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.MethodNotFoundCode, "method not allowed")
			return
		}
		next.ServeHTTP(w, r)
	})
}