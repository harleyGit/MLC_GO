/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-18 17:01:25
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-07-24 17:54:28
 * @FilePath: /MLC_GO/TestNotes/unfamiliar_grammar_practice/nsq_project_practice/nsq_project_main.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package nsq_project_practice

import (
	"MLC_GO/TestNotes/unfamiliar_grammar_practice/nsq_project_practice/nsq_practice_v1"

	"github.com/gin-gonic/gin"
)

func NSQProjectPracticeMain() {
	nsqPracticeV1 :=  nsq_practice_v1.NSQPracticeV1{}
	nsqPracticeV1.ExecutePracticeNone()
	nsqPracticeV1. NSQPracticeV1()

	nsqPracticeV1.NSQPraCMDParse()

}

func ginSetup() {
	gin.SetMode(gin.DebugMode)
}