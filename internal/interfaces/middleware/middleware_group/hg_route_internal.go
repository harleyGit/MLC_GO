package HGMiddlewareGroupPackage

import "net/http"

// hgRouteSpec 描述模块内子路由注册元信息。
type hgRouteSpec struct {
	Group    string
	Method   string
	SubPath  string
	FullPath string
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
		mux.HandleFunc(route.SubPath, route.Handler)
	}
}

// buildRouteCatalogItems 负责把子路由元信息转成完整对外可调用路径清单。
func buildRouteCatalogItems(specs []hgRouteSpec) []HGRouteCatalogItem {
	items := make([]HGRouteCatalogItem, 0, len(specs))
	for _, spec := range specs {
		items = append(items, HGRouteCatalogItem{
			Group:    spec.Group,
			Method:   spec.Method,
			Path:     spec.FullPath,
			NeedAuth: spec.NeedAuth,
			Summary:  spec.Summary,
		})
	}

	return items
}

// newRouteSpec 统一构建路由元信息，确保子路径和完整路径同时可用。
func newRouteSpec(
	group string,
	method string,
	basePath string,
	subPath string,
	needAuth bool,
	summary string,
	handler http.HandlerFunc,
) hgRouteSpec {
	return hgRouteSpec{
		Group:    group,
		Method:   method,
		SubPath:  subPath,
		FullPath: joinRoutePath(basePath, subPath),
		NeedAuth: needAuth,
		Summary:  summary,
		Handler:  handler,
	}
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
