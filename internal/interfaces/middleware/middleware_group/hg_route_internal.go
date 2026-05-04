/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-04-30 22:06:12
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-05-04 13:22:47
 * @FilePath: /MLC_GO/internal/interfaces/middleware/middleware_group/hg_route_internal.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
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

/*
	bindRouteSpecs 负责把模块内子路由批量注册到 mux。

批量注册路由：遍历路由规格列表，将每个子路由注册到 `http.ServeMux`。
*/
func bindRouteSpecs(mux *http.ServeMux, specs []hgRouteSpec) {
	for _, route := range specs {
		if route.Handler == nil {
			continue
		}
		mux.HandleFunc(route.SubPath, route.Handler) // 注册路由
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
/* newRouteSpec 统一构建路由元信息，确保子路径和完整路径同时可用。
	 	group: 									分组名
    method: 								HTTP 方法，比如：http.MethodGet
    basePath: 							完整前缀，比如："/api/v1/profile"
    subPath: 								子路径，比如："/info"
    needAuth: 							是否需要认证
    summary: 								描述,"获取用户信息"
    handler: 								处理函数,比如：userHandler.Profile
*/

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
