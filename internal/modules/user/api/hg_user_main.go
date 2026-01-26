/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-22 21:16:00
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-26 00:31:40
 * @FilePath: /MLC_GO/internal/modules/user/api/hg_user_main.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package UserAPIPackage

import (
	PersistenceSQLPackage "MLC_GO/internal/infrastructure/persistence/mysql"
	PersistenceRedisPackage "MLC_GO/internal/infrastructure/persistence/redis"
	HGMiddlewarePackage "MLC_GO/internal/interfaces/middleware"
	HGSMSPackage "MLC_GO/internal/modules/sms"
	UserHandlerPackage "MLC_GO/internal/modules/user/handler"
	UserJWTMiddlewarePackage "MLC_GO/internal/modules/user/middleware"
	"MLC_GO/internal/pkg/logHG"
	"log"
	"net/http"
	"time"
)

// 废弃
func UserMainV3() {

	// --- Redis ----
	redisService := PersistenceRedisPackage.NewRedisService() // 初始化Redis连接

	// -- MySQL----
	sqlManager, err := PersistenceSQLPackage.NewSQLManager()
	if err != nil {
		logHG.ErrFInfo("数据库初化失败，error：", err)
		return
	}

	//--------- 依赖 ----------
	smsSender := HGSMSPackage.NewMockSender()
	userHandler := UserHandlerPackage.NewUserHandler(redisService, sqlManager, smsSender)

	// ----- 路由---------
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/send_code", userHandler.SendCode)
	mux.HandleFunc("/user/register", userHandler.RegisterHandlerV3)
	mux.HandleFunc("/auth/login", userHandler.Login)
	// 这里调用 AuthMiddleware，鉴权调用 会每次调用的时候都调用的
	mux.Handle("/profile", UserJWTMiddlewarePackage.AuthMiddleware(http.HandlerFunc(userHandler.Profile)))

	// --- server ---------
	srv := &http.Server{
		Addr:         ":8080",
		Handler:      HGMiddlewarePackage.HGCORSMiddleware(mux),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	logHG.DebugInfo("HTTP server 开始监听： 8080 ")
	log.Fatal(srv.ListenAndServe())
}

func UserMainV2() {
	_, err := PersistenceSQLPackage.NewSQLDB()
	if err != nil {
		logHG.FatalFInfo("数据库初化失败，error：", err)
	}

	UserHandlerPackage.RegisterUserRoutesV2()
	srv := http.Server{
		Addr:         ":8080",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	logHG.DebugInfo("Starting server on :8080")
	log.Fatal(srv.ListenAndServe())
}
