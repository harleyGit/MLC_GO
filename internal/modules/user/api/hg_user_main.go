/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-22 21:16:00
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-02-01 11:40:37
 * @FilePath: /MLC_GO/internal/modules/user/api/hg_user_main.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package UserAPIPackage

import (
	PersistenceSQLPackage "MLC_GO/internal/pkg/mysql"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	HGUserModulePackage "MLC_GO/internal/modules/user/module"
	"MLC_GO/internal/pkg/logHG"
	"log"
	"net/http"
	"time"
)

func UserMainV2() {
	db, err := PersistenceSQLPackage.NewSQLManager()
	if err != nil {
		logHG.FatalFInfo("数据库初化失败，error：", err)
	}
	redisService := PersistenceRedisPackage.NewRedisService()

	HGUserModulePackage.RegisterModules(redisService, db, nil)
	srv := http.Server{
		Addr:         ":8080",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	logHG.DebugInfo("Starting server on :8080")
	log.Fatal(srv.ListenAndServe())
}
