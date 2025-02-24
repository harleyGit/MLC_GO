/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-02-23 21:47:11
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-02-24 17:58:52
 * @FilePath: /MLC_GO/TestNotes/PracticeGenExample/pkg/util/pagination.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package util

import (
	"MLC_GO/TestNotes/PracticeGenExample/pkg/setting"

	"github.com/gin-gonic/gin"
	"github.com/unknwon/com"
)

/*
gin.Context 是 Gin 框架中用于处理 HTTP 请求和响应的上下文对象。它的主要作用包括：

获取请求信息：如请求头、请求体、URL 参数、查询参数等。
设置响应信息：如响应状态码、响应头、响应体等。
中间件传递数据：可以在中间件中向 gin.Context 中添加数据，然后在后续的处理函数中获取这些数据。
控制请求流程：可以通过 c.Abort()、c.Next() 等方法控制请求的处理流程。
*/
func GetPage(c *gin.Context) int {
	result := 0
	// com.StrTo("123").MustInt() 可以将字符串 "123" 转换为整数 123
	// 获取 URL 查询参数。如：请求 URL：/search?q=gin
	// q := c.Query("q")
	page, _ := com.StrTo(c.Query("page")).Int()
	if page > 0 {
		result = (page -1) * setting.PageSize
	}

	return result
}