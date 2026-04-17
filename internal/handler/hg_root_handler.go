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
	HGPracticeTestHandlerPackage "MLC_GO/TestNotes/handler_practice"
	PersistenceSQLPackage "MLC_GO/internal/infrastructure/persistence/mysql"
	PersistenceRedisPackage "MLC_GO/internal/infrastructure/persistence/redis"
	HGMiddlewareGroupPackage "MLC_GO/internal/interfaces/middleware/middleware_group"
	HGSMSPackage "MLC_GO/internal/modules/sms"
	UserHandlerPackage "MLC_GO/internal/modules/user/handler"
	"net/http"
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
	publicHandler := HGMiddlewareGroupPackage.AuthMiddlewareGoup(userHandler)
	userHandlerWithAuth := HGMiddlewareGroupPackage.UserMiddlewareGoup(userHandler)
	testHandler := HGPracticeTestHandlerPackage.PracticeTestHandler()

	registerRootPrefixRoutes(rootMux, []HGRouteMount{
		// 统一前缀，便于网关治理与版本演进
		{Prefix: "/api/v1/auth/", StripPrefix: "/api/v1/auth", Handler: publicHandler},
		{Prefix: "/api/v1/user/", StripPrefix: "/api/v1/user", Handler: publicHandler},
		{Prefix: "/api/v1/profile/", StripPrefix: "/api/v1/profile", Handler: userHandlerWithAuth},
		{Prefix: "/api/v1/test/", StripPrefix: "/api/v1/test", Handler: testHandler},
	})

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

/*
路由访问：

| 请求 URL                   | 实际命中                      |
| ------------------------ | ------------------------- |
| `/api/v1/auth/send_code` | `publicMux -> /send_code` |
| `/api/v1/auth/login`     | `publicMux -> /login`     |
| `/api/v1/user/register`  | `publicMux -> /register`  |
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
