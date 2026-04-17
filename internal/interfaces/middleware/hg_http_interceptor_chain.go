package HGMiddlewarePackage

import "net/http"

// HGHTTPInterceptor 表示一个可组合的 HTTP 拦截器。
type HGHTTPInterceptor func(http.Handler) http.Handler

// ChainInterceptors 在启动阶段把拦截器链装配为最终 handler，避免请求期重复构建。
func ChainInterceptors(base http.Handler, interceptors ...HGHTTPInterceptor) http.Handler {
	if base == nil {
		return nil
	}

	wrapped := base
	for i := len(interceptors) - 1; i >= 0; i-- {
		if interceptors[i] == nil {
			continue
		}
		wrapped = interceptors[i](wrapped)
	}

	return wrapped
}
