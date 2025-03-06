/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-02-28 20:10:02
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-06 18:02:29
 * @FilePath: /MLC_GO/TestNotes/PracticeGenExample/routers/api/v1/article.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package v1

import (
	"MLC_GO/TestNotes/GenPracticeExample/models"
	"MLC_GO/TestNotes/GenPracticeExample/pkg/e"
	"MLC_GO/TestNotes/GenPracticeExample/pkg/logging"
	"MLC_GO/TestNotes/GenPracticeExample/pkg/setting"
	"MLC_GO/TestNotes/GenPracticeExample/pkg/util"
	"net/http"

	"github.com/astaxie/beego/validation"
	"github.com/gin-gonic/gin"
	"github.com/unknwon/com"
)

// 获取单个文章
func GetArticle(c *gin.Context) {
	id := com.StrTo(c.Param("id")).MustInt()

    valid := validation.Validation{}
	valid.Min(id, 1, "id").Message("ID必须大于0")

	code := e.INVALID_PARAMS
	var data interface{}
	if ! valid.HasErrors() {
		if models.ExistArticleByID(id) {
			data = models.GetArticle(id)
			code = e.SUCCESS
		} else {
			code = e.ERROR_NOT_EXIST_ARTICLE
		}
	} else {
		for _, err := range valid.Errors {
			logging.Info("err.key: %s, err.message: %s", err.Key, err.Message)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code": code,
		"msg": e.GetMsg(code),
		"data": data,
	})
}


// 获取多个文章
func GetArticles(c *gin.Context ){
	data := make(map[string]interface{})
	maps := make(map[string]interface{})
	valid := validation.Validation{}

	var state int = -1
	if arg := c.Query("state"); arg != "" {
		state = com.StrTo(arg).MustInt()
		maps["state"] = state

		valid.Range(state, 0, 1, "state").Message("状态只允许0或1")
	}

	var tagId int = -1
	if  arg := c.Query("tag_id"); arg != "" {
		tagId =  com.StrTo(arg).MustInt()
		maps["tag_id"] = tagId

		valid.Min(tagId, 1, "tag_id").Message("标签ID必须大于0")
	}

	code := e.INVALID_PARAMS
	if ! valid.HasErrors() {
		code = e.SUCCESS

		data["lists"] = models.GetArticles(util.GetPage(c), setting.PageSize, maps)
		data["total"] = models.GetArticleTotal(maps)
	} else {
		for  _, err := range valid.Errors {
			logging.Info("err.key: %s, err.message: %s", err.Key, err.Message)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code" : code,
		"msg": e.GetMsg(code),
		"data": data,
	})
}

/* 新增文章
c *gin.Context 是 Gin 框架提供的上下文对象，包含了请求、响应、请求参数等相关信息。*gin.Context 允许你访问请求中的所有数据，比如查询参数、表单数据、路径参数、请求头等。
 */
 func AddArticle(c *gin.Context) {
	/* c.Query("tag_id")：c.Query() 是从 URL 查询字符串中提取参数的方法，"tag_id" 是你在 URL 中传递的查询参数名。如果请求的 URL 是 /add?tag_id=1，那么 c.Query("tag_id") 将返回字符串 "1"。
	com.StrTo(...)：com.StrTo 是一个库函数（通常来自第三方库，如 github.com/unknwon/com），它将字符串转换为其他类型。这行代码首先把查询字符串转换为 StrTo 类型
	MustInt()：这是 com.StrTo 类型的一个方法，将字符串转换为整数。如果转换失败，MustInt() 会触发 panic（错误处理会中断程序）。它是强制转换方法，假设该参数总是能被正确转换为整数
	 */
	tagId := com.StrTo(c.Query("tag_id")).MustInt()
	title := c.Query("title")
	desc := c.Query("desc")
	content := c.Query("content")
	createdBy := c.Query("created_by")
	state := com.StrTo(c.DefaultQuery("state", "0")).MustInt()

	valid := validation.Validation{}
	valid.Min(tagId, 1, "tag_id").Message("标签ID必须大于0")
	valid.Required(title, "title").Message("标签不能为空")
	valid.Required(desc, "desc").Message("简述不能为空")
	valid.Required(content, "content").Message("内容不能为空")
	valid.Required(createdBy, "createdBy").Message("创建人不能为空")
	valid.Range(state, 0, 1, "state").Message("状态只允许0或1")

	code := e.INVALID_PARAMS
	if ! valid.HasErrors() {
		if exists, err := models.ExistTagByID(tagId); err == nil && exists  {
			data := make(map[string]interface {})
			data["tag_id"] = tagId
			data["title"] = title
            data["desc"] = desc
            data["content"] = content
            data["created_by"] = createdBy
            data["state"] = state

			models.AddArticle(data)
			code = e.SUCCESS
		}else {
			code = e.ERROR_NOT_EXIST_TAG
		}
	}else {
		for _, err := range valid.Errors {
			logging.Info("err.key: %s, err.message: %s", err.Key, err.Message)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code": code,
		"msg": e.GetMsg(code),
		"data": make(map[string]interface {}),
	})
}

// 修改文章
func EditArticle(c *gin.Context) {
	valid := validation.Validation{}

	id := com.StrTo(c.Param("id")).MustInt()
	tagId := com.StrTo(c.Query("tag_id")).MustInt()
	title := c.Query("title")
    desc := c.Query("desc")
    content := c.Query("content")
    modifiedBy := c.Query("modified_by")

	var state int = -1
	if arg := c.Query("state"); arg != "" {
		state = com.StrTo(arg).MustInt()
		valid.Range(state, 0, 1, "state").Message("状态只允许0或1")
	}

	valid.Min(id, 1, "id").Message("ID必须大于0")
    valid.MaxSize(title, 100, "title").Message("标题最长为100字符")
    valid.MaxSize(desc, 255, "desc").Message("简述最长为255字符")
    valid.MaxSize(content, 65535, "content").Message("内容最长为65535字符")
    valid.Required(modifiedBy, "modified_by").Message("修改人不能为空")
    valid.MaxSize(modifiedBy, 100, "modified_by").Message("修改人最长为100字符")

	code := e.INVALID_PARAMS
	if ! valid.HasErrors() {
		if models.ExistArticleByID(tagId) {
			/* make 是 Go 语言中的一个内建函数，用于创建并初始化切片（slice）、映射（map）和通道（channel）。在这个例子中，make 被用来创建一个 map。

			map[string]interface{} 是 map 类型的声明，表示一个键是 string 类型，值是 interface{} 类型的映射。
 			*/
			data := make(map[string]interface {})
			if tagId > 0 {
				data["tag_id"] = tagId
			}
			if  title != "" {
				data["title"] = title
			}
			if desc != "" {
				data["desc"] = desc
			}
			if content != "" {
				data["content"] = content
			}

			data["modified_by"] = modifiedBy

			models.EditArticle(id, data)
			code = e.SUCCESS
		}else {
			code = e.ERROR_NOT_EXIST_ARTICLE
		}
	}else {
		for _, err := range valid.Errors {
            logging.Debug("err.key: %s, err.message: %s", err.Key, err.Message)
        }
	}

	c.JSON(http.StatusOK, gin.H{
        "code" : code,
        "msg" : e.GetMsg(code),
        "data" : make(map[string]string),
    })
}

// 删除文章
func DeleteArticle(c *gin.Context) {
	id := com.StrTo(c.Param("id")).MustInt()

	valid := validation.Validation{}
	valid.Min(id, 1, "id").Message("ID必须大于0")

	code := e.INVALID_PARAMS
	if ! valid.HasErrors() {
		if models.ExistArticleByID(id) {
			models.DeleteArticle(id)
			code = e.SUCCESS
		}else {
			code = e.ERROR_NOT_EXIST_ARTICLE
		}
	}else {
		for _, err := range valid.Errors {
            logging.Debug("err.key: %s, err.message: %s", err.Key, err.Message)
        }
	}

	c.JSON(http.StatusOK, gin.H{
        "code" : code,
        "msg" : e.GetMsg(code),
        "data" : make(map[string]string),
    })
}