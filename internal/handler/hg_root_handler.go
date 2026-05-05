/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-26 20:06:46
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-05-05 08:26:58
 * @FilePath: /MLC_GO/internal/handler/hg_root_handler.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package HGHandlerPackage

import (
	PersistenceSQLPackage "MLC_GO/internal/infrastructure/persistence/mysql"
	PersistenceRedisPackage "MLC_GO/internal/infrastructure/persistence/redis"
	HGMiddlewarePackage "MLC_GO/internal/interfaces/middleware"
	HGMiddlewareGroupPackage "MLC_GO/internal/interfaces/middleware/middleware_group"
	HGSMSPackage "MLC_GO/internal/modules/sms"
	HGTestHandlerPackage "MLC_GO/internal/modules/test/handler"
	UserCachePackage "MLC_GO/internal/modules/user/cache"
	UserHandlerPackage "MLC_GO/internal/modules/user/handler"
	UserRepositoryPackage "MLC_GO/internal/modules/user/repository"
	UserServicePackage "MLC_GO/internal/modules/user/service"
	"MLC_GO/internal/pkg/logHG"
	HGResponsePakcage "MLC_GO/internal/response"
	"net/http"
	"sort"
)

// HGRootHandlerDeps 统一承载 Root 路由装配所需依赖，避免入口函数参数持续膨胀。
type HGRootHandlerDeps struct {
	RedisService *PersistenceRedisPackage.RedisService
	SQLManager   *PersistenceSQLPackage.HGSQLManager
	SMSSender    HGSMSPackage.HGSender
}

// HGRouteMount 定义一个模块路由挂载点，便于按模块扩展和统一管理前缀策略。
type HGRouteMount struct {
	Prefix      string
	StripPrefix string
	Handler     http.Handler
}

const routeCatalogPath = "/api/v1/routes"
const routeCatalogGroupsPath = "/api/v1/routes/groups"

/*
	若是使用路由可以使用httprouter库，gin它们底层用的都是它。
*/
// NewRootHandler 负责构建根路由，仅挂载 /api/v1 前缀的模块路由。
func NewRootHandler(deps HGRootHandlerDeps) *http.ServeMux {
	// 根路由只负责挂载模块路由，具体路径由各模块定义，确保模块内路径清晰且不受外层变动影响。
	rootMux := http.NewServeMux()

	smsSender := deps.SMSSender
	if smsSender == nil {
		smsSender = HGSMSPackage.NewMockSender()
	}

	// 依赖注入：创建 Repository、Cache、Service，然后注入到 Handler
	db := deps.SQLManager.GetSQLDB()
	redisClient := UserCachePackage.NewCodeCache(deps.RedisService)
	userCache := UserCachePackage.NewUserCache(deps.RedisService)
	userRepo := UserRepositoryPackage.NewUserRepo(db)
	svc := UserServicePackage.NewUserService(userRepo, userCache, deps.RedisService)
	tokenService := UserServicePackage.NewAuthService(userRepo, redisClient)

	userHandler := UserHandlerPackage.NewUserHandler(deps.RedisService, svc, tokenService, smsSender)
	publicHandler := HGMiddlewareGroupPackage.NewAuthRouteInterceptorGroup(userHandler)
	userHandlerWithAuth := HGMiddlewareGroupPackage.NewUserRouteInterceptorGroup(userHandler)

	testHandler := HGTestHandlerPackage.TestModuleHandler()
	// API 调用路径清单
	routeCatalog := buildRouteCatalog()
	// 按模块分组
	routeCatalogGrouped := buildRouteCatalogGrouped(routeCatalog)

	// 挂载模块路由
	registerRootPrefixRoutes(rootMux, []HGRouteMount{
		// 统一前缀，便于网关治理与版本演进
		{
			Prefix:      HGMiddlewareGroupPackage.AuthModuleBasePath + "/",
			StripPrefix: HGMiddlewareGroupPackage.AuthModuleBasePath,
			Handler:     publicHandler,
		},
		{
			Prefix:      HGMiddlewareGroupPackage.UserProfileModuleBasePath + "/",
			StripPrefix: HGMiddlewareGroupPackage.UserProfileModuleBasePath,
			Handler:     userHandlerWithAuth,
		},
		{
			Prefix:      HGTestHandlerPackage.TestModuleBasePath + "/",
			StripPrefix: HGTestHandlerPackage.TestModuleBasePath,
			Handler:     testHandler,
		},
	})
	// rootMux.Handle 注册路由清单接口
	// 可以没有，但有了更方便
	// 没有路由清单的情况
	// 后端写好接口 → 前端开发人员问"有哪些接口？" → 后端口头告诉 / 写文档
	rootMux.Handle(routeCatalogPath, buildRouteCatalogHandler(newRouteCatalogHandler(routeCatalog)))
	rootMux.Handle(routeCatalogGroupsPath, buildRouteCatalogHandler(newRouteCatalogGroupedHandler(routeCatalogGrouped)))

	// 启动时打印路由清单
	logRouteCatalog(routeCatalog)

	return rootMux
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

// buildRouteCatalog 汇总完整 API 调用路径清单，供 App/Web 联调用。
func buildRouteCatalog() []HGMiddlewareGroupPackage.HGRouteCatalogItem {
	items := make([]HGMiddlewareGroupPackage.HGRouteCatalogItem, 0, 16)
	items = append(items, HGMiddlewareGroupPackage.AuthRouteCatalog()...)
	items = append(items, HGMiddlewareGroupPackage.UserRouteCatalog()...)
	items = append(items, HGTestHandlerPackage.TestRouteCatalog()...)
	items = append(items, HGMiddlewareGroupPackage.HGRouteCatalogItem{
		Group:    "meta",
		Method:   http.MethodGet,
		Path:     routeCatalogPath,
		NeedAuth: false,
		Summary:  "查看完整 API 路由清单",
	})
	items = append(items, HGMiddlewareGroupPackage.HGRouteCatalogItem{
		Group:    "meta",
		Method:   http.MethodGet,
		Path:     routeCatalogGroupsPath,
		NeedAuth: false,
		Summary:  "按模块分组查看 API 路由清单",
	})

	// sort.Slice` 是 Go 标准库的排序函数：
	// 参数 1：要排序的切片
	// 参数 2：比较函数，返回 `true` 表示 `i` 应该排在 `j` 前面
	sort.Slice(items, func(i, j int) bool {
		// 第一步：先按 Path 排序
		if items[i].Path == items[j].Path {
			// 第二步：Path 相同时，按 Method 排序
			return items[i].Method < items[j].Method
		}
		// 默认：按 Path 字母顺序排序
		return items[i].Path < items[j].Path
	})

	return items
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
