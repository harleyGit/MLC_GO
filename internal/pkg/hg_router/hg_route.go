package HGRouterPackage

import (
	"net/http"
	"slices"
)

// region 数据结构定义

// RouteCatalogItem 描述对外可调用的 API 路由，供 App/Web 联调查看。
type RouteCatalogItem struct {
	Group           string   `json:"group"`                     // 业务模块名。
	Method          string   `json:"method"`                    // HTTP 方法。
	Path            string   `json:"path"`                      // 包含模块前缀的路径。
	NeedAuth        bool     `json:"needAuth"`                  // 是否需要可信登录身份。
	Summary         string   `json:"summary"`                   // 接口用途。
	Permissions     []string `json:"permissions,omitempty"`     // IAM 权限编码，默认全部满足；空表示不额外要求全局 RBAC。
	PermissionMatch string   `json:"permissionMatch,omitempty"` // all 或 any；仅非空权限列表有效。
}

// HGRouteCatalogItem 兼容旧类型名。
type HGRouteCatalogItem = RouteCatalogItem

// RouteSpec 描述模块内子路由注册元信息。
type RouteSpec struct {
	Group           string           // 业务模块名。
	Method          string           // HTTP 方法。
	SubPath         string           // StripPrefix 后的模块内路径。
	FullPath        string           // 对外完整路径。
	NeedAuth        bool             // 是否需要登录；声明权限时必须为 true。
	Summary         string           // 接口用途。
	Handler         http.HandlerFunc // 业务入口；目录构建时可为 nil。
	Permissions     []string         // JWT 验证后检查的 IAM 权限，不能代替业务资源归属校验。
	PermissionMatch string           // 默认 all；any 只用于明确允许任一权限的共享入口。
}

// hgRouteSpec 兼容旧类型名。
type hgRouteSpec = RouteSpec

// endregion

// region 路由注册工具

// BindRouteSpecs 负责把模块内子路由批量注册到 mux。
func BindRouteSpecs(mux *http.ServeMux, specs []RouteSpec, hgAuthorizers ...HGRouteAuthorizer) {
	for _, route := range specs {
		if route.Handler == nil {
			continue
		}
		mux.Handle(route.Method+" "+route.SubPath, hgPermissionHandler(route, hgFirstAuthorizer(hgAuthorizers)))
	}
}

// bindRouteSpecs 兼容旧方法名。
func bindRouteSpecs(mux *http.ServeMux, specs []RouteSpec) {
	BindRouteSpecs(mux, specs)
}

// BuildRouteCatalogItems 负责把子路由元信息转成完整对外可调用路径清单。
func BuildRouteCatalogItems(specs []RouteSpec) []RouteCatalogItem {
	items := make([]RouteCatalogItem, 0, len(specs))
	for _, spec := range specs {
		items = append(items, RouteCatalogItem{
			Group:           spec.Group,
			Method:          spec.Method,
			Path:            spec.FullPath,
			NeedAuth:        spec.NeedAuth,
			Summary:         spec.Summary,
			Permissions:     slices.Clone(spec.Permissions),
			PermissionMatch: spec.PermissionMatch,
		})
	}

	return items
}

// buildRouteCatalogItems 兼容旧方法名。
func buildRouteCatalogItems(specs []RouteSpec) []RouteCatalogItem {
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
	hgPermissions ...string,
) RouteSpec {
	hgMatch := ""
	if len(hgPermissions) > 0 {
		hgMatch = "all"
	}
	return RouteSpec{
		Group:           group,
		Method:          method,
		SubPath:         subPath,
		FullPath:        JoinRoutePath(basePath, subPath),
		NeedAuth:        needAuth,
		Summary:         summary,
		Handler:         handler,
		Permissions:     slices.Clone(hgPermissions),
		PermissionMatch: hgMatch,
	}
}

// HGWithAnyPermission 将共享查询入口声明为任一权限可访问；写操作应使用默认 all。
func (hgSpec RouteSpec) HGWithAnyPermission() RouteSpec {
	hgSpec.PermissionMatch = "any"
	return hgSpec
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

// endregion
