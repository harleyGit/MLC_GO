package HGMiddlewareGroupPackage

import "net/http"

// hgRouteSpec 描述模块内子路由注册元信息。
type hgRouteSpec struct {
	Method   string
	Path     string
	NeedAuth bool
	Summary  string
	Handler  http.HandlerFunc
}

func joinRoutePath(prefix string, subPath string) string {
	if prefix == "" {
		return subPath
	}
	if subPath == "" || subPath == "/" {
		return prefix
	}
	if subPath[0] != '/' {
		return prefix + "/" + subPath
	}

	return prefix + subPath
}
