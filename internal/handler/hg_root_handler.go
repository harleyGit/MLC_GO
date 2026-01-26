/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-26 20:06:46
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-26 20:35:02
 * @FilePath: /MLC_GO/internal/handler/hg_root_handler.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package HGHandlerPackage

import (
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

	rootMux := http.NewServeMux()
	userHandler := UserHandlerPackage.NewUserHandler(redisService, sqlManager, smsSender)
	publicHandler := HGMiddlewareGroupPackage.AuthMiddlewareGoup(userHandler)
	userHandlerWithAuth := HGMiddlewareGroupPackage.UserMiddlewareGoup(userHandler)

	// public
	rootMux.Handle("/auth/", publicHandler)
	rootMux.Handle("/user/register", publicHandler)

	// user
	rootMux.Handle("/profile", userHandlerWithAuth)
	rootMux.Handle("user", userHandlerWithAuth)

	// order
	// rootMux.Handle("/order/", orderHandleWithAuth)

	return rootMux
}
