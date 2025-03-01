package v1

import (
	"MLC_GO/TestNotes/PracticeGenExample/models"
	"MLC_GO/TestNotes/PracticeGenExample/pkg/e"
	"MLC_GO/TestNotes/PracticeGenExample/pkg/setting"
	"MLC_GO/TestNotes/PracticeGenExample/pkg/util"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/unknwon/com"
)

// 获取多个文章标签
// c *gin.Context是Gin很重要的组成部分，可以理解为上下文，它允许我们在中间件之间传递变量、管理流、验证请求的 JSON 和呈现 JSON 响应
func GetTags(c *gin.Context){
	// c.Query可用于获取?name=test&state=1这类 URL 参数，而c.DefaultQuery则支持设置一个默认值
	name := c.Query("name")

	maps := make(map[string]interface{})
	data := make(map[string]interface{})

	if name != "" {
		maps["name"] = name
	}

	var state int = -1
	if arg := c.Query("state"); arg != "" {
		state = com.StrTo(arg).MustInt()
		maps["state"] = state
	}

	// 使用了e模块的错误编码，这正是先前规划好的错误码，方便排错和识别记录
	code := e.SUCCESS

	// util.GetPage保证了各接口的page处理是一致的
	data["lists"] = models.GetTags(util.GetPage(c), setting.PageSize, maps)
	data["total"] = models.GetTagTotal(maps)

	c.JSON(http.StatusOK, gin.H{
		"code": code,
		"msg": e.GetMsg(code),
		"data": data,
	})
}



// 新增文章标签
func AddTag(c *gin.Context){

}

// 修改文章标签
func EditTag(c *gin.Context){}

// 删除文章标签
func DeleteTag(c *gin.Context){

}