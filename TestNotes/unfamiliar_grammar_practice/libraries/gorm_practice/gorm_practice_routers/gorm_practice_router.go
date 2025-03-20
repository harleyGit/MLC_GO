/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-19 21:53:55
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-19 22:11:31
 * @FilePath: /MLC_GO/TestNotes/unfamiliar_grammar_practice/libraries/gorm_practice/gorm_practice_routers/gorm_practice_router.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package gorm_practice_routers

import (
	"MLC_GO/pkg/hg_uuid"
	"MLC_GO/pkg/hglog"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// 路由设置
func SetupRouters() *gin.Engine {

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(testGlobalMiddleWare())

	return router
}


// 全局中间件示例
func testGlobalMiddleWare() gin.HandlerFunc {
	return func(c *gin.Context) {
		hglog.DebugInfo("MiddleWare: 中间件开始执行")

		// 在gin.Context中设置一个值 演示中间件的能力
		traceId, _ := hg_uuid.GenerateUUID()
		c.Set("trace_id", traceId)

		//todo 这里你可以执行你想做的任何事情

		// 执行完这里的逻辑之后别忘了 调用 Next 函数将请求交给下个 handler 处理
		c.Next()

		status := c.Writer.Status()
		hglog.DebugInfo("MiddleWare: 中间件执行结束, status: ", zap.Any("status", status))
	}
}
