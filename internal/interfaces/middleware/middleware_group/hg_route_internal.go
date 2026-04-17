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

type hgMiddleware func(http.Handler) http.Handler

// bindRouteSpecs 负责把模块内子路由批量注册到 mux。
func bindRouteSpecs(mux *http.ServeMux, specs []hgRouteSpec) {
	for _, route := range specs {
		if route.Handler == nil {
			continue
		}
		mux.HandleFunc(route.Path, route.Handler)
	}
}

// buildRouteCatalogItems 负责把子路由元信息转成完整对外可调用路径清单。
func buildRouteCatalogItems(group string, basePrefix string, specs []hgRouteSpec) []HGRouteCatalogItem {
	items := make([]HGRouteCatalogItem, 0, len(specs))
	for _, spec := range specs {
		items = append(items, HGRouteCatalogItem{
			Group:    group,
			Method:   spec.Method,
			Path:     joinRoutePath(basePrefix, spec.Path),
			NeedAuth: spec.NeedAuth,
			Summary:  spec.Summary,
		})
	}

	return items
}

// chainMiddlewares 按声明顺序拼接中间件，避免手工嵌套导致链路顺序不清晰。
func chainMiddlewares(base http.Handler, chain ...hgMiddleware) http.Handler {
	if base == nil {
		return nil
	}

	wrapped := base
	for i := len(chain) - 1; i >= 0; i-- {
		if chain[i] == nil {
			continue
		}
		wrapped = chain[i](wrapped)
	}

	return wrapped
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
