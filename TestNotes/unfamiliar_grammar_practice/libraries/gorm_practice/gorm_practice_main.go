/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-19 13:14:02
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-20 15:55:21
 * @FilePath: /MLC_GO/TestNotes/unfamiliar_grammar_practice/libraries/gorm_practice/gorm_practice_main.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package gorm_practice

import (
	"MLC_GO/TestNotes/unfamiliar_grammar_practice/libraries/gorm_practice/gorm_practice_routers"
	"MLC_GO/TestNotes/unfamiliar_grammar_practice/libraries/gorm_practice/gorm_practice_v"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func GormPracticeMain() {
	gormPracticeV1 := gorm_practice_v.GormPracticeV1{}
	gormPracticeV1.ExecutePracticeNone()

	// Gorm 初始化数据库并产生数据库全局变量
	gormPracticeV1.GormPracticeV1_connect()

	gin.SetMode(gin.DebugMode)

	routersInit := gorm_practice_routers.SetupRouters()

	server := &http.Server{
		Addr:           ":8080",
		Handler:        routersInit,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	server.ListenAndServe()
}