package HGMiddlewarePackage

import (
	HGResponsePakcage "MLC_GO/internal/response"
	"net/http"
)

type HGMethodRule struct {
	Path    string
	Methods map[string]bool
}

type HGMethodGuard struct {
	rules map[string]HGMethodRule
}

func NewMethodGuard(rules []HGMethodRule) *HGMethodGuard {

	ruleMap := make(map[string]HGMethodRule)
	for _, r := range rules {
		ruleMap[r.Path] = r
	}

	return &HGMethodGuard{rules: ruleMap}
}

func MethodGuardMiddlewareV2(rules []HGMethodRule) func(http.Handler) http.Handler {

	ruleMap := make(map[string]HGMethodRule)
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

func (m *HGMethodGuard) MethodGuardMiddleware(next http.Handler) http.Handler {

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
