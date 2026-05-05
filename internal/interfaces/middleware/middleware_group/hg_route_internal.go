package HGMiddlewareGroupPackage

import "net/http"

// RouteSpec 描述模块内子路由注册元信息。
type RouteSpec struct {
	Group    string
	Method   string
	SubPath  string
	FullPath string
	NeedAuth bool
	Summary  string
	Handler  http.HandlerFunc
}

// hgRouteSpec 兼容旧类型名。
type hgRouteSpec = RouteSpec

// BindRouteSpecs 负责把模块内子路由批量注册到 mux。
func BindRouteSpecs(mux *http.ServeMux, specs []RouteSpec) {
	for _, route := range specs {
		if route.Handler == nil {
			continue
		}
		mux.HandleFunc(route.SubPath, route.Handler)
	}
}

// bindRouteSpecs 兼容旧方法名。
func bindRouteSpecs(mux *http.ServeMux, specs []RouteSpec) {
	BindRouteSpecs(mux, specs)
}

// BuildRouteCatalogItems 负责把子路由元信息转成完整对外可调用路径清单。
func BuildRouteCatalogItems(specs []RouteSpec) []HGRouteCatalogItem {
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

// buildRouteCatalogItems 兼容旧方法名。
func buildRouteCatalogItems(specs []RouteSpec) []HGRouteCatalogItem {
	return BuildRouteCatalogItems(specs)
}

// NewRouteSpec 统一构建路由元信息，确保子路径和完整路径同时可用。
func NewRouteSpec(
	group string,
	method string,
	basePath string,
	subPath string,
	needAuth bool,
	summary string,
	handler http.HandlerFunc,
) RouteSpec {
	return RouteSpec{
		Group:    group,
		Method:   method,
		SubPath:  subPath,
		FullPath: JoinRoutePath(basePath, subPath),
		NeedAuth: needAuth,
		Summary:  summary,
		Handler:  handler,
	}
}

// newRouteSpec 兼容旧方法名。
func newRouteSpec(
	group string,
	method string,
	basePath string,
	subPath string,
	needAuth bool,
	summary string,
	handler http.HandlerFunc,
) RouteSpec {
	return NewRouteSpec(group, method, basePath, subPath, needAuth, summary, handler)
}

// JoinRoutePath 拼接路由路径。
func JoinRoutePath(prefix string, subPath string) string {
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

// joinRoutePath 兼容旧方法名。
func joinRoutePath(prefix string, subPath string) string {
	return JoinRoutePath(prefix, subPath)
}
