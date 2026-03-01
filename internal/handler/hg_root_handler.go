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

func RootHander(redisService *PersistenceRedisPackage.RedisService,
	sqlManager *PersistenceSQLPackage.HGSQLManager,
	smsSender *HGSMSPackage.HGMockerSender) *http.ServeMux {

	// rootMux 里只出现 /auth/、/user/这种前缀
	rootMux := http.NewServeMux()
	userHandler := UserHandlerPackage.NewUserHandler(redisService, sqlManager, smsSender)
	publicHandler := HGMiddlewareGroupPackage.AuthMiddlewareGoup(userHandler)
	userHandlerWithAuth := HGMiddlewareGroupPackage.UserMiddlewareGoup(userHandler)
	testHandler := HGPracticeTestHandlerPackage.PracticeTestHandler()

	// public【前缀】
	rootMux.Handle("/auth/", http.StripPrefix("/auth", publicHandler))
	rootMux.Handle("/user/", http.StripPrefix("/user", publicHandler))

	// user【前缀】
	rootMux.Handle("/profile/", http.StripPrefix("/profile", userHandlerWithAuth))

	// order
	// rootMux.Handle("/order/", orderHandleWithAuth)

	// 测试
	rootMux.Handle("/test/", http.StripPrefix("/test", testHandler))

	return rootMux
}

/*
路由访问：

| 请求 URL            | 实际命中                      |
| ----------------- | ------------------------- |
| `/auth/send_code` | `publicMux -> /send_code` |
| `/auth/login`     | `publicMux -> /login`     |
| `/user/register`  | `publicMux -> /register`  |
| `/profile`        | `userMux -> /`            |
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
