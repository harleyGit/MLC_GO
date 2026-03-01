/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-02-01 12:30:27
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-02-26 10:05:41
 * @FilePath: /MLC_GO/internal/interfaces/middleware/hg_method_guard_middle.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package HGMiddlewarePackage

import (
	UtilsPackage "MLC_GO/internal/pkg/utils"
	HGResponsePakcage "MLC_GO/internal/response"
	"context"
	"errors"
	"net/http"
	"strings"
)

type HGAPIRule struct {
	Path        string
	Methods     map[string]bool
	NeedAuth    bool
	Permissions []string
	Version     string
}

type ctxKey string

const (
	CtxUserID   ctxKey = "userID"
	CtxDeviceID ctxKey = "deviceID"
)

type HGAPIGuard struct {
	rules map[string]HGAPIRule
}

var rolePermissions = map[string]map[string]bool{
	"admin": {
		"user:update": true,
		"user:view":   true,
	},
	"user": {
		"user:view": true,
	},
}

func NewAPIGuard(rules []HGAPIRule) *HGAPIGuard {

	ruleMap := make(map[string]HGAPIRule)
	for _, r := range rules {
		key := r.Version + r.Path
		ruleMap[key] = r
	}

	return &HGAPIGuard{rules: ruleMap}
}

func (g *HGAPIGuard) MethodGuardMiddlewareV3(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		version := r.Header.Get("X-API-Version")
		if version == "" {
			version = "v1"
		}

		key := version + r.URL.Path
		rule, ok := g.rules[key]
		if !ok {
			http.NotFound(w, r)
			return
		}

		// 1️⃣ Method 校验
		if !rule.Methods[r.Method] {
			w.WriteHeader(http.StatusMethodNotAllowed)
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.MethodNotAllowCode, "method not allowed")
			return
		}

		//2️⃣ 登录态校验
		if rule.NeedAuth {
			if r.Context().Value("uid") == nil {
				w.WriteHeader(http.StatusUnauthorized)
				HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.UnauthorizedCode, "not unauthorized")
				return
			}
		}

		// 2.1 header检验
		ctx := g.checkoutHeader(w, r)
		if ctx == nil {
			return
		}

		// 3️⃣ 权限校验（可选）
		if len(rule.Permissions) > 0 {
			role, _ := r.Context().Value("role").(string)
			if !HasPermission(role, rule.Permissions) {
				w.WriteHeader(http.StatusForbidden)
				HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.ForbiddenCode, "FORBIDDEN")
				return
			}
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (g *HGAPIGuard) checkoutHeader(w http.ResponseWriter, r *http.Request) context.Context {

	// ===== 1. Header 校验 =====
	token := r.Header.Get("Authorization")            // Authorization: Bearer <access_token>
	conentType := r.Header.Get("Content-Type")        // Content-Type: application/json
	deviceID := r.Header.Get("X-Device-ID")           // X-Device-ID: a1b2c3d4-e5f6-7890
	// clientType := r.Header.Get("X-Client-Type")       // X-Client-Type: ios / android / web
	// clientVersion := r.Header.Get("X-Client-Version") // 设备版本号：1.0.0
	// version := r.Header.Get("X-API-Version")          // X-Client-Version: 2.1.0
	// languange := r.Header.Get("X-Language")           // Accept-Language: zh-CN,zh;q=0.9,en;q=0.8 【想要 简体中文（中国） 的内容；如果没有，其他中文也行；如果连中文都没有，那就给我 英文 吧； q 是 “quality factor”（质量因子）的缩写，取值范围是 0.0 ~ 1.0，数值越大，优先级越高】
	// requestid := r.Header.Get("X-Request-ID")         // 【添加请求 ID（用于日志追踪）：】X-Request-ID: abc123def456
	// timestamp := r.Header.Get("X-Timestamp")          // X-Timestamp: 1700000000
	// signature := r.Header.Get("X-Signature")          // TODO：后端验证签名，防止中间人伪造请求，请求体 + 时间戳 + 密钥进行 HMAC 签名，放入 Header：X-Signature: sha256=8f42a1b3c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2

	if UtilsPackage.IsEmpty(token) ||
		UtilsPackage.IsEmpty(conentType){
		//||
		// UtilsPackage.IsEmpty(deviceID) ||
		// UtilsPackage.IsEmpty(clientType) ||
		// UtilsPackage.IsEmpty(clientVersion) ||
		// UtilsPackage.IsEmpty(version) ||
		// UtilsPackage.IsEmpty(languange) ||
		// UtilsPackage.IsEmpty(requestid) ||
		// UtilsPackage.IsEmpty(timestamp) 
		
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.RuequestHeaderNotOk, HGResponsePakcage.RequestHeaderFailDesc)
		return nil
	}

	userID, err := verfiyToken(token)
	if err != nil {
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.TokenInvalidCode, HGResponsePakcage.TokenInvalidFailDesc)
		return nil
	}
	// ===== 2. 写入 Context =====
	ctx := context.WithValue(r.Context(), CtxUserID, userID)
	ctx = context.WithValue(ctx, CtxDeviceID, deviceID)

	return ctx
}

func verfiyToken(token string) (string, error) {

	if !strings.HasPrefix(token, "Bearer ") {
		return "0", errors.New("invalid token format")
		
	}
// 模拟解析 token，JWT 实际项目中应该调用认证服务进行验证
	return "", nil
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

		rule, ok := m.rules[r.URL.Path]
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
