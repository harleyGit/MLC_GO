/*
* @Author: GangHuang harleysor@qq.com
* @Date: 2025-02-27 12:55:15
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-02 15:56:42

* @FilePath: /MLC_GO/TestNotes/PracticeGenExample/routers/router.go
* @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE

* 注册路由
*/
package routers

import (
	"MLC_GO/TestNotes/PracticeGenExample/middleware/jwt"
	"MLC_GO/TestNotes/PracticeGenExample/routers/api"
	v1 "MLC_GO/TestNotes/PracticeGenExample/routers/api/v1"

	"github.com/gin-gonic/gin"
)

func InitRouter() *gin.Engine {
	/*
	gin.New() 创建了一个新的 gin 路由实例 (Engine)，相当于初始化一个 Web 服务器。
	gin.New() 不会默认加载任何中间件（如日志、恢复等），如果需要默认中间件，可以使用 gin.Default()。
	*/
	r := gin.New()

	r.Use(gin.Logger())

	r.Use(gin.Recovery())
	
	r.GET("/auth", api.GetAuth)
	// 定义了一个 API 路由组 apiv1，所有在这个组内的路由都会有 "/api/v1" 这个前缀。
	apiv1 := r.Group("/api/v1")
	/* 
	作用：
		JWT() 返回一个 Gin 中间件（gin.HandlerFunc）。
		这个中间件会在请求到达具体的路由前执行，比如检查 JWT 是否有效。

		若是执行终止后续处理：ctx.Abort() ，则下面的 apiv1.GET("/tags", v1.GetTags)等都不会执行了，否则继续执行。
		若是ctx.Next() 让 apiv1.GET("/tags", v1.GetTags)继续执行
	*/
	apiv1.Use(jwt.JWT())
	{
		// 获取标签列表
		// 定义了一个 GET 请求的路由，完整的 URL 访问路径为: GET /api/v1/tags
		// v1.GetTags 这个函数应该是一个 HTTP 处理函数（handler），用于处理 GET /api/v1/tags 请求，通常返回一个 JSON 响应，包含标签列表的数据。
		apiv1.GET("/tags", v1.GetTags)
		// 新建标签
		apiv1.POST("/tags", v1.AddTag)
		// 更新指定标签
		apiv1.PUT("/tags/:id", v1.EditTag)
		// 删除指定标签
		apiv1.DELETE("/tags/:id", v1.DeleteTag)


		// 获取文章列表
		// 这是定义一个 GET 请求的路由。当客户端发起 GET 请求到 /articles 路径时，Gin 会将请求转发到处理函数 v1.GetArticles。
		apiv1.GET("/articles", v1.GetArticles)
		// 获取指定文章
		apiv1.GET("/articles/:id",v1.GetArticle)
		// 新建文章
		apiv1.POST("/articles", v1.AddArticle)
		// 更新指定文章
		apiv1.PUT("/articles/:id", v1.EditArticle)
		// 删除指定文章
		apiv1.DELETE("/articles/:id", v1.DeleteArticle)
		// 生成文章海报
		// apiv1.POST("/articles/poster/generate", v1.generateArticlePoster)
	}

	return r
}