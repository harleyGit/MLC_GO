/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-19 21:53:55
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-20 17:00:23
 * @FilePath: /MLC_GO/TestNotes/unfamiliar_grammar_practice/libraries/gorm_practice/gorm_practice_routers/gorm_practice_router.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package gorm_practice_routers

import (
	"MLC_GO/TestNotes/unfamiliar_grammar_practice/libraries/gorm_practice/gorm_practice_routers/gorm_router_api"
	"MLC_GO/pkg/hg_uuid"
	"MLC_GO/pkg/hglog"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)
var (
	// log    = config.GVA_LOG
	origin = "www.baidu.com"
)

// 路由设置
func SetupRouters() *gin.Engine {

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(globalMiddleWare())
	router.Use(gormPracticeCORS())

	// 给表单限制上传大小 (default is 32 MiB)
	router.MaxMultipartMemory = 8 << 20 // 8 MiB
	apiUser := router.Group("/api/user")
	apiUser.Use()
	{
		// 新增用户表单请求
		apiUser.POST("/addUser", gorm_router_api.AddUser)
		// 新增用户json请求
		apiUser.POST("/addUserUseJson", gorm_router_api.AddUserUseJson)

		//根据uid查询用户信息
		apiUser.GET("/getUserByUid", gorm_router_api.GetUserByUid)
		//根据uid查询用户信息 - 参数在请求路径中
		apiUser.GET("/getUserByUid/:uid", gorm_router_api.GetUserByUidUseRouteParam)
	}


	return router
}


// 全局中间件示例
func globalMiddleWare() gin.HandlerFunc {
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
//添加跨域支持
func gormPracticeCORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		if origin != "" {
			// 可将将* 替换为指定的域名
			c.Header("Access-Control-Allow-Origin", "*")
			c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, UPDATE")
			c.Header("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, Authorization")
			c.Header("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers, Cache-Control, Content-Language, Content-Type")
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		if method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
		}

		c.Next()
	}
}

