/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-19 18:46:51
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-19 21:34:47
 * @FilePath: /MLC_GO/TestNotes/unfamiliar_grammar_practice/libraries/gin_practice/gin_practice_main.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package gin_practice

import "MLC_GO/TestNotes/unfamiliar_grammar_practice/libraries/gin_practice/gin_practice_v"


func GinPracticeMain() {
	
	// Gin 框架的日志功能:日志输出到指定文件夹
	gin_practice_v.GormPracticeV1_v8()

	// 添加中间件
	// gin_practice_v.GormPracticeV1_v7()

	// 静态文件服务：展示了如何在 Gin 框架中提供静态文件服务，可以方便地将静态资源文件（如图片、样式表、脚本等）提供给客户端。
	// gin_practice_v.GormPracticeV1_v6()

	// 使用 LoadHTMLGlob 方法加载了位于 "templates" 目录下的所有模板文件。
	// gin_practice_v.GormPracticeV1_v5()
	
	// 参数化路由和路由组
	// gin_practice_v.GormPracticeV1_v4()

	// 接收 JSON 格式的请求体，并将其绑定到结构体中进行处理
	// gin_practice_v.GormPracticeV1_v3()
	
	// 加入中间件、参数解析、日志记录等
	// gin_practice_v.GormPracticeV1_v2()

	// 普通测试
	// gin_practice_v.GormPracticeV1_v1()
}