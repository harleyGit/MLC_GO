/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-02-01 12:30:27
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-02-01 16:42:42
 * @FilePath: /MLC_GO/internal/interfaces/middleware/hg_method_guard_middle.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package HGMiddlewarePackage

import (
	HGResponsePakcage "MLC_GO/internal/response"
	"net/http"
)

type HGAPIRule struct {
	Path        string
	Methods     map[string]bool
	NeedAuth    bool
	Permissions []string
	Version     string
}

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

		// 3️⃣ 权限校验（可选）
		if len(rule.Permissions) > 0 {
			role, _ := r.Context().Value("role").(string)
			if !HasPermission(role, rule.Permissions) {
				w.WriteHeader(http.StatusForbidden)
				HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.ForbiddenCode, "FORBIDDEN")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
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
