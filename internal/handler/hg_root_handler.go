/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-26 20:06:46
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-05-05 21:42:05
 * @FilePath: /MLC_GO/internal/handler/hg_root_handler.go
 * @Description: 根路由处理器，支持模块自注册模式
 */
package HGHandlerPackage

import (
	HGMiddlewarePackage "MLC_GO/internal/interfaces/middleware"
	HGMiddlewareGroupPackage "MLC_GO/internal/interfaces/middleware/middleware_group"
	"MLC_GO/internal/pkg/logHG"
	HGResponsePakcage "MLC_GO/internal/response"
	"net/http"
	"sort"
)

const routeCatalogPath = "/api/v1/routes"
const routeCatalogGroupsPath = "/api/v1/routes/groups"

// HGRouteMount 定义一个模块路由挂载点，便于按模块扩展和统一管理前缀策略。
type HGRouteMount struct {
	Prefix      string
	StripPrefix string
	Handler     http.Handler
}

/*
	若是使用路由可以使用httprouter库，gin它们底层用的都是它。
*/
// NewRootHandler 负责构建根路由，从模块注册表中获取所有模块并挂载。
func NewRootHandler(routeCatalogs []HGMiddlewareGroupPackage.HGRouteCatalogItem) *http.ServeMux {
	// 根路由只负责挂载模块路由，具体路径由各模块定义，确保模块内路径清晰且不受外层变动影响。
	rootMux := http.NewServeMux()

	// 从注册表获取所有模块并挂载路由
	mounts := make([]HGRouteMount, 0, 8)
	for _, mod := range GetRegisteredModules() {
		mounts = append(mounts, HGRouteMount{
			Prefix:      mod.BasePath() + "/",
			StripPrefix: mod.BasePath(),
			Handler:     mod.Handler(), // 根据各自的模块，加入中间件+拦截器
		})
	}
	registerRootPrefixRoutes(rootMux, mounts)

	// API 调用路径清单
	catalog := routeCatalogs
	// 添加元数据路由（路由清单接口）
	catalog = appendMetaRoutes(catalog)
	// 按模块分组
	catalogGrouped := buildRouteCatalogGrouped(catalog)

	// rootMux.Handle 注册路由清单接口
	// 可以没有，但有了更方便
	// 没有路由清单的情况
	// 后端写好接口 → 前端开发人员问"有哪些接口？" → 后端口头告诉 / 写文档
	rootMux.Handle(routeCatalogPath, buildRouteCatalogHandler(newRouteCatalogHandler(catalog)))
	rootMux.Handle(routeCatalogGroupsPath, buildRouteCatalogHandler(newRouteCatalogGroupedHandler(catalogGrouped)))

	// 启动时打印路由清单
	logRouteCatalog(catalog)

	return rootMux
}

// appendMetaRoutes 添加元数据路由（路由清单接口）。
func appendMetaRoutes(catalog []HGMiddlewareGroupPackage.HGRouteCatalogItem) []HGMiddlewareGroupPackage.HGRouteCatalogItem {
	catalog = append(catalog, HGMiddlewareGroupPackage.HGRouteCatalogItem{
		Group:    "meta",
		Method:   http.MethodGet,
		Path:     routeCatalogPath,
		NeedAuth: false,
		Summary:  "查看完整 API 路由清单",
	})
	catalog = append(catalog, HGMiddlewareGroupPackage.HGRouteCatalogItem{
		Group:    "meta",
		Method:   http.MethodGet,
		Path:     routeCatalogGroupsPath,
		NeedAuth: false,
		Summary:  "按模块分组查看 API 路由清单",
	})

	// sort.Slice` 是 Go 标准库的排序函数：
	// 参数 1：要排序的切片
	// 参数 2：比较函数，返回 `true` 表示 `i` 应该排在 `j` 前面
	sort.Slice(catalog, func(i, j int) bool {
		// 第一步：先按 Path 排序
		if catalog[i].Path == catalog[j].Path {
			// 第二步：Path 相同时，按 Method 排序
			return catalog[i].Method < catalog[j].Method
		}
		// 默认：按 Path 字母顺序排序
		return catalog[i].Path < catalog[j].Path
	})

	return catalog
}

// registerRootPrefixRoutes 统一处理前缀挂载，确保各模块的 strip-prefix 行为一致。
func registerRootPrefixRoutes(rootMux *http.ServeMux, mounts []HGRouteMount) {
	for _, mount := range mounts {
		if mount.Handler == nil || mount.Prefix == "" || mount.StripPrefix == "" {
			continue
		}
		rootMux.Handle(mount.Prefix, http.StripPrefix(mount.StripPrefix, mount.Handler))
	}
}

/*
	把扁平的路由列表按 Group 字段分成多个组，每组内部再按 Path 排序

buildRouteCatalogGrouped 把路由按 group 聚合，便于 App/Web 按模块展示。
*/
func buildRouteCatalogGrouped(catalog []HGMiddlewareGroupPackage.HGRouteCatalogItem) map[string][]HGMiddlewareGroupPackage.HGRouteCatalogItem {
	// 1️⃣ 创建分组 map
	grouped := make(map[string][]HGMiddlewareGroupPackage.HGRouteCatalogItem, 8)
	// 2️⃣ 按 Group 字段分组
	for _, item := range catalog {
		grouped[item.Group] = append(grouped[item.Group], item)
	}

	// 3️⃣ 每组内部排序
	for group := range grouped {
		routes := grouped[group]
		sort.Slice(routes, func(i, j int) bool {
			if routes[i].Path == routes[j].Path {
				return routes[i].Method < routes[j].Method
			}
			return routes[i].Path < routes[j].Path
		})
		grouped[group] = routes
	}

	return grouped
}

// logRouteCatalog 在服务启动时输出完整 API 路由清单，方便 App/Web 联调时直接确认完整路径。
func logRouteCatalog(catalog []HGMiddlewareGroupPackage.HGRouteCatalogItem) {
	logHG.DebugInfo("API 路由清单如下（完整可调用路径）：")
	for _, item := range catalog {
		logHG.DebugFInfo("[API] %s %s auth=%t group=%s summary=%s", item.Method, item.Path, item.NeedAuth, item.Group, item.Summary)
	}
	logHG.DebugFInfo("API 路由清单接口：GET %s", routeCatalogPath)
	logHG.DebugFInfo("API 路由分组接口：GET %s", routeCatalogGroupsPath)
}

// newRouteCatalogHandler 提供完整接口路径查询，方便 App/Web 联调自助查看。
func newRouteCatalogHandler(catalog []HGMiddlewareGroupPackage.HGRouteCatalogItem) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 路由清单是只读接口，只允许 GET 方法，其他方法返回 405
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			HGResponsePakcage.FailResult[string](
				w,
				r,
				HGResponsePakcage.MethodNotAllowCode,
				"method not allowed",
			)
			return
		}

		HGResponsePakcage.SuccessResult(w, r, catalog)
	})
}

// newRouteCatalogGroupedHandler 提供按模块分组的路由清单，方便端侧按业务域筛选。
func newRouteCatalogGroupedHandler(grouped map[string][]HGMiddlewareGroupPackage.HGRouteCatalogItem) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			HGResponsePakcage.FailResult[string](
				w,
				r,
				HGResponsePakcage.MethodNotAllowCode,
				"method not allowed",
			)
			return
		}

		HGResponsePakcage.SuccessResult(w, r, grouped)
	})
}

func buildRouteCatalogHandler(core http.Handler) http.Handler {
	return HGMiddlewarePackage.ChainInterceptors(
		core,
		HGMiddlewarePackage.RecoverInterceptor,
		HGMiddlewarePackage.AccessLogInterceptor,
		HGMiddlewarePackage.RequestTIDInterceptor,
		HGMiddlewarePackage.JSONHeaderInterceptor,
	)
}

/*
路由访问：

| 请求 URL                   | 实际命中                      |
| ------------------------ | ------------------------- |
| `/api/v1/auth/send_code` | `publicMux -> /send_code` |
| `/api/v1/auth/login`     | `publicMux -> /login`     |
| `/api/v1/auth/register`  | `publicMux -> /register`  |
| `/api/v1/profile/info`   | `userMux -> /info`        |
*/

/* 现在请求链路是这样的：
Request
 ↓
RequestTIDInterceptor
 ↓
AccessLogInterceptor
 ↓
RecoverInterceptor
 ↓
JSONHeaderInterceptor
 ↓
APIGuardInterceptor   ← Method / Auth / Permission / Version
 ↓
(User 模块额外) JWT AuthInterceptor
 ↓
Handler
 ↓
Service

*/
