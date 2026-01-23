/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-22 21:16:00
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-23 14:44:29
 * @FilePath: /MLC_GO/internal/modules/user/api/hg_user_main.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package UserAPIPackage

import (
	HGSMSPackage "MLC_GO/internal/modules/sms"
	UserHandlerPackage "MLC_GO/internal/modules/user/handler"
	UserJWTMiddlewarePackage "MLC_GO/internal/modules/user/middleware"
	"MLC_GO/internal/pkg/logHG"
	"context"
	"log"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

func UserMain() {
	// --- Redis ----
	rdb := redis.NewClient(&redis.Options{
		Addr:     "127.0.0.1:6379",
		Password: "",
		DB:       0,
	})

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		logHG.FatalFInfo("redis 连接失败:", err)
	}

	//--------- 依赖 ----------
	smsSender := &HGSMSPackage.HGMockerSender{}
	userHandler := UserHandlerPackage.NewUserHandler(rdb, nil, smsSender)

	// ----- 路由---------
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/send_code", userHandler.SendCode)
	mux.HandleFunc("/auth/login", userHandler.Login)
	// 这里调用 AuthMiddleware，鉴权调用 会每次调用的时候都调用的
	mux.Handle("/profile", UserJWTMiddlewarePackage.AuthMiddleware(http.HandlerFunc(userHandler.Profile)))

	// --- server ---------
	srv := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	logHG.DebugInfo("HTTP server 开始监听： 8080 ")
	log.Fatal(srv.ListenAndServe())

}

// eat: 🍒 1.直接运行 main.go
// 2. 真实短信接口；
// 3.审计日志+风控日志（登录、验证码、刷接口）
