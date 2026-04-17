/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-26 20:06:46
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-02-07 20:43:40
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
	UserHandlerPackage "MLC_GO/internal/modules/user/handler"
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

// NewRootHandler 负责构建根路由，仅挂载 /api/v1 前缀的模块路由。
func NewRootHandler(deps HGRootHandlerDeps) *http.ServeMux {
	rootMux := http.NewServeMux()

	smsSender := deps.SMSSender
	if smsSender == nil {
		smsSender = HGSMSPackage.NewMockSender()
	}

	userHandler := UserHandlerPackage.NewUserHandler(deps.RedisService, deps.SQLManager, smsSender)
	publicHandler := HGMiddlewareGroupPackage.AuthMiddlewareGroup(userHandler)
	userHandlerWithAuth := HGMiddlewareGroupPackage.UserMiddlewareGroup(userHandler)
	testHandler := HGTestHandlerPackage.TestModuleHandler()
	routeCatalog := buildRouteCatalog()

	registerRootPrefixRoutes(rootMux, []HGRouteMount{
		// 统一前缀，便于网关治理与版本演进
		{Prefix: "/api/v1/auth/", StripPrefix: "/api/v1/auth", Handler: publicHandler},
		{Prefix: "/api/v1/profile/", StripPrefix: "/api/v1/profile", Handler: userHandlerWithAuth},
		{Prefix: "/api/v1/test/", StripPrefix: "/api/v1/test", Handler: testHandler},
	})
	rootMux.Handle(
		"/api/v1/routes",
		HGMiddlewarePackage.JSONHeaderMiddleware(
			HGMiddlewarePackage.TIDMiddleware(
				newRouteCatalogHandler(routeCatalog),
			),
		),
	)

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
	items = append(items, HGMiddlewareGroupPackage.AuthRouteCatalog("/api/v1/auth")...)
	items = append(items, HGMiddlewareGroupPackage.UserRouteCatalog("/api/v1/profile")...)
	items = append(items, HGTestHandlerPackage.TestRouteCatalog("/api/v1/test")...)
	items = append(items, HGMiddlewareGroupPackage.HGRouteCatalogItem{
		Group:    "meta",
		Method:   http.MethodGet,
		Path:     "/api/v1/routes",
		NeedAuth: false,
		Summary:  "查看完整 API 路由清单",
	})

	sort.Slice(items, func(i, j int) bool {
		if items[i].Path == items[j].Path {
			return items[i].Method < items[j].Method
		}
		return items[i].Path < items[j].Path
	})

	return items
}

// newRouteCatalogHandler 提供完整接口路径查询，方便 App/Web 联调自助查看。
func newRouteCatalogHandler(catalog []HGMiddlewareGroupPackage.HGRouteCatalogItem) http.Handler {
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

		HGResponsePakcage.SuccessResult(w, r, catalog)
	})
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
APIGuardMiddleware   ← Method / Auth / Permission / Version
 ↓
Recover
 ↓
Logger
 ↓
TID
 ↓
JSONHeader
 ↓
Handler
 ↓
Service

*/
