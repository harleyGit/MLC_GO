/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-22 21:16:00
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-22 21:28:59
 * @FilePath: /MLC_GO/internal/modules/user/api/hg_user_main.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package UserAPIPackage

import (
	HGSMSPackage "MLC_GO/internal/modules/sms"
	HGSMSCachePackage "MLC_GO/internal/modules/sms/cache"
	UserHandlerPackage "MLC_GO/internal/modules/user/handler"
	"MLC_GO/internal/pkg/logHG"
	"context"

	"github.com/redis/go-redis/v9"
)


func UserMain() {
	// --- Redis ----
	rdb := redis.NewClient(&redis.Options{
		Addr: "127.0.0.1:6379",
		Password: "",
		DB: 0,
	})

	if  err := rdb.Ping(context.Background()).Err(); err != nil {
		logHG.FatalFInfo("redis 连接失败:", err)
	}

	//--------- 依赖 ----------
	smsSender := HGSMSPackage.HGMockerSender{}
	userHandler := UserHandlerPackage.NewUserHandler(rdb, smsSender)
}
