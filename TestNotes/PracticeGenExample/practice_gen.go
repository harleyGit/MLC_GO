/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-02-23 17:05:53
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-02-23 17:08:25
 * @FilePath: /MLC_GO/TestNotes/PracticeGen/practice_gen_test.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package main

import "github.com/gin-gonic/gin"

func main()  {
	practiceGenPing()
}

func practiceGenPing() {//另起一个终端程序，命令： curl 127.0.0.1:8080/ping
	r := gin.Default()
	r.GET("/ping", func(ctx *gin.Context) {
		ctx.JSON(200, gin.H{
			"message": "pong",
		})
	})
	r.Run() // listen and serve on 0.0.0.0:8080
}