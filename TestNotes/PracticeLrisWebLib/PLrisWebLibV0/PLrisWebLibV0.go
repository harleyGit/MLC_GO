/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-02-02 11:46:28
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-02-02 11:59:44
 * @FilePath: /MLC_GO/TestNotes/PracticeLrisWebLib/PLrisWebLibV0/PLrisWebLibV0.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
 // title: Lris Web 启动项目
package main

import "github.com/kataras/iris/v12"

func main() {
	practiceIrisWebLibV0_get()
}

func practiceIrisWebLibV0_get() {
	// 默认服务引擎
	app := iris.Default()
	// 路由： /ping
	// 控制逻辑： 返回JSON格式内容
	app.Get("/ping", func (ctx iris.Context)  {
		ctx.JSON(iris.Map{
			"message": "pong",
		})
	})
	// listen and serve on http://0.0.0.0:8080/ping
	app.Run(iris.Addr(":8080"))
}