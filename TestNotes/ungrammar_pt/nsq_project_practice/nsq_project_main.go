/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-18 17:01:25
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-09-11 16:53:13
 * @FilePath: /MLC_GO/TestNotes/unfamiliar_grammar_practice/nsq_project_practice/nsq_project_main.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package nsq_project_practice

import (
	"MLC_GO/TestNotes/ungrammar_pt/nsq_project_practice/nsq_practice_v1"

	"github.com/gin-gonic/gin"
)

func NSQProjectPracticeMain() {
	nsqPracticeV1 :=  nsq_practice_v1.NSQPracticeV1{}
	nsqPracticeV1.ExecutePracticeNone()

	// 装饰器测试: http://localhost:8080/ping → 返回 pong
	// http://localhost:8080/hello/Alice → 返回 Hello, Alice!
	nsqPracticeV1.Decorate_pt_NSQ()

	// 上下文取消
	// nsqPracticeV1.NSQCancelContext()
	
	// 日志打印
	// nsqPracticeV1.NSQCustomLog()
	
	//nsqPracticeV1. NSQPracticeV1()

	// 命令行参数解析
	// nsqPracticeV1.NSQPraCMDParse()

	// 反射解析命令行参数
	//nsq_practice_v1.PT_NSQReflect00()

	// 路径获取
	//nsqPracticeV1.NSQFilePathPT()
}

func ginSetup() {
	gin.SetMode(gin.DebugMode)
}