/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-02-28 20:10:02
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-13 18:59:42
 * @FilePath: /MLC_GO/TestNotes/PracticeGenExample/routers/api/v1/article.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package v1

import (
	"MLC_GO/TestNotes/GenPracticeExample/models"
	"MLC_GO/TestNotes/GenPracticeExample/pkg/app"
	"MLC_GO/TestNotes/GenPracticeExample/pkg/e"
	"MLC_GO/TestNotes/GenPracticeExample/pkg/logging"
	"MLC_GO/TestNotes/GenPracticeExample/pkg/setting"
	"MLC_GO/TestNotes/GenPracticeExample/pkg/util"
	"MLC_GO/TestNotes/GenPracticeExample/service/article_service"
	"MLC_GO/TestNotes/GenPracticeExample/service/tag_service"
	"net/http"

	"github.com/astaxie/beego/validation"
	"github.com/gin-gonic/gin"
	"github.com/unknwon/com"
)

// 获取单个文章
func GetArticle(c *gin.Context) {
	// 通过包装 c 到自定义的 app.Gin 对象中，可以在后续代码中通过 appG 来访问 gin.Context 以及可能扩展的其它方法和属性。
	appG := app.Gin{c}
	// 使用 c.Param("id") 从请求的 URL 中提取名为 "id" 的参数，返回的是字符串类型。
	// com.StrTo(...) 是一个辅助函数，用于将字符串转换为其它数据类型。
	// 调用 .MustInt() 将该字符串转换成整数，如果转换失败则会引发异常（或返回默认值，取决于具体实现）。
	id := com.StrTo(c.Param("id")).MustInt()
	// 创建一个 validation.Validation 实例，通常来自于 Beego Validation 或类似的验证库，用于对数据进行规则校验。
	valid := validation.Validation{}
	// 第一个参数 id：待验证的数值（从 URL 参数转换得到）。
	// 第二个参数 1：验证条件，即 id 的最小值要求必须大于或等于 1。
	// 第三个参数 "id"：标识符或字段名称，用于错误信息中区分验证失败的字段。
	// 调用 .Message("ID必须大于0")：
	// 为验证失败的情况设置一个自定义的错误提示信息，告诉用户或开发者 "ID必须大于0"。
	valid.Min(id, 1, "id").Message("ID必须大于0")

	if valid.HasErrors() {
		app.MarkErrors(valid.Errors)
		appG.Response(http.StatusOK, e.INVALID_PARAMS, nil)
		return
	}

	articleService := article_service.Article{ID: id}
	exists, err := articleService.ExistByID()
	if err != nil {
		appG.Response(http.StatusOK, e.ERROR_CHECK_EXIST_ARTICLE_FAIL, nil)
		return
	}
	if !exists {
		appG.Response(http.StatusOK, e.ERROR_NOT_EXIST_ARTICLE, nil)
		return
	}

	article, err := articleService.Get()
	if err != nil {
		appG.Response(http.StatusOK, e.ERROR_GET_ARTICLE_FAIL, nil)
		return
	}

	appG.Response(http.StatusOK, e.SUCCESS, article)
}

// 获取单个文章
// Deprecated: func GetArticle_v1(c *gin.Context)废弃了,请用 func GetArticle(c *gin.Context)
func GetArticle_v1(c *gin.Context) {
	id := com.StrTo(c.Param("id")).MustInt()

    valid := validation.Validation{}
	valid.Min(id, 1, "id").Message("ID必须大于0")

	code := e.INVALID_PARAMS
	var data interface{}
	if ! valid.HasErrors() {
		if isExistArticle, _ := models.ExistArticleByID(id); isExistArticle {
			data,_ = models.GetArticle(id)
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

		data["lists"] = models.GetArticles(util.GetPage(c), setting.AppSetting.PageSize, maps)
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

type AddArticleForm struct {
	TagID         int    `form:"tag_id" valid:"Required;Min(1)"`
	Title         string `form:"title" valid:"Required;MaxSize(100)"`
	Desc          string `form:"desc" valid:"Required;MaxSize(255)"`
	Content       string `form:"content" valid:"Required;MaxSize(65535)"`
	CreatedBy     string `form:"created_by" valid:"Required;MaxSize(100)"`
	CoverImageUrl string `form:"cover_image_url" valid:"Required;MaxSize(255)"`
	State         int    `form:"state" valid:"Range(0,1)"`
}
// @Summary Add article
// @Produce  json
// @Param tag_id body int true "TagID"
// @Param title body string true "Title"
// @Param desc body string true "Desc"
// @Param content body string true "Content"
// @Param created_by body string true "CreatedBy"
// @Param state body int true "State"
// @Success 200 {object} app.Response
// @Failure 500 {object} app.Response
// @Router /api/v1/articles [post]
func AddArticle(c *gin.Context) {
	var (
		appG = app.Gin{C: c}
		form AddArticleForm
	)

	httpCode, errCode := app.BindAndValid(c, &form)
	if errCode != e.SUCCESS {
		appG.Response(httpCode, errCode, nil)
		return
	}

	tagService := tag_service.Tag{ID: form.TagID}
	exitsts, err := tagService.ExistByID()
	if err != nil {
		appG.Response(http.StatusInternalServerError, e.ERROR_EXIST_TAG_FAIL, nil)
		return
	}

	if !exitsts {//不存在
		logging.DebugInfo("该博客文章已经存在了")
		// appG.Response(http.StatusOK, e.ERROR_NOT_EXIST_TAG, nil)
		// return
	}

	articleService := article_service.Article {
		TagID:         form.TagID,
		Title:         form.Title,
		Desc:          form.Desc,
		Content:       form.Content,
		CoverImageUrl: form.CoverImageUrl,
		State:         form.State,
		CreatedBy:     form.CreatedBy,
	}

	if err := articleService.Add(); err != nil {
		appG.Response(http.StatusInternalServerError, e.ERROR_ADD_ARTICLE_FAIL, nil) 
		return
	}

	appG.Response(http.StatusOK, e.SUCCESS, "新建博客成功!!")

}

/* 新增文章
c *gin.Context 是 Gin 框架提供的上下文对象，包含了请求、响应、请求参数等相关信息。*gin.Context 允许你访问请求中的所有数据，比如查询参数、表单数据、路径参数、请求头等。
 */
 // Deprecated: 该方法不再使用,请用 func AddArticleV1(c *gin.Context) 
 func AddArticleV1(c *gin.Context) {
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

type EditArticleForm struct {
	ID            int    `form:"id" valid:"Required;Min(1)"`
	TagID         int    `form:"tag_id" valid:"Required;Min(1)"`
	Title         string `form:"title" valid:"Required;MaxSize(100)"`
	Desc          string `form:"desc" valid:"Required;MaxSize(255)"`
	Content       string `form:"content" valid:"Required;MaxSize(65535)"`
	ModifiedBy    string `form:"modified_by" valid:"Required;MaxSize(100)"`
	CoverImageUrl string `form:"cover_image_url" valid:"Required;MaxSize(255)"`
	State         int    `form:"state" valid:"Range(0,1)"`
}
// @Summary Update article
// @Produce  json
// @Param id path int true "ID"
// @Param tag_id body string false "TagID"
// @Param title body string false "Title"
// @Param desc body string false "Desc"
// @Param content body string false "Content"
// @Param modified_by body string true "ModifiedBy"
// @Param state body int false "State"
// @Success 200 {object} app.Response
// @Failure 500 {object} app.Response
// @Router /api/v1/articles/{id} [put]
func EditArticle(c *gin.Context) {
	var (
		appG = app.Gin{C: c}
		form = EditArticleForm{ID: com.StrTo(c.Param("id")).MustInt()}
	)

	httpCode, errCode := app.BindAndValid(c, &form)
	if errCode != e.SUCCESS {
		appG.Response(httpCode, errCode, "参数不准确")
		return
	}

	articleService := article_service.Article{
		ID:            form.ID,
		TagID:         form.TagID,
		Title:         form.Title,
		Desc:          form.Desc,
		Content:       form.Content,
		CoverImageUrl: form.CoverImageUrl,
		ModifiedBy:    form.ModifiedBy,
		State:         form.State,
	}
	exists, err := articleService.ExistByID()
	if err != nil {
		appG.Response(http.StatusInternalServerError, e.ERROR_CHECK_EXIST_ARTICLE_FAIL, nil)
		return
	}
	if !exists {
		appG.Response(http.StatusOK, e.ERROR_NOT_EXIST_ARTICLE, "不存在该篇博客")
		return
	}

	// 这段不能加的原因是blog_tag表没有数据,这段逻辑会有问题
	// tagService := tag_service.Tag{ID: form.TagID}
	// exists, err = tagService.ExistByID()
	// if err != nil {
	// 	appG.Response(http.StatusInternalServerError, e.ERROR_EXIST_TAG_FAIL, nil)
	// 	return
	// }

	// if !exists {
	// 	appG.Response(http.StatusOK, e.ERROR_NOT_EXIST_TAG, nil)
	// 	return
	// }

	err = articleService.Edit()
	if err != nil {
		appG.Response(http.StatusInternalServerError, e.ERROR_EDIT_ARTICLE_FAIL, nil)
		return
	}

	appG.Response(http.StatusOK, e.SUCCESS, "博客修改成功")

}

// 修改文章
// Deprecated: 该方法不再使用,请用 func EditArticleV1(c *gin.Context) 
func EditArticleV1(c *gin.Context) {
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
		if isExistArticle, _ := models.ExistArticleByID(tagId); isExistArticle {
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
		if isExistArticle, _ := models.ExistArticleByID(id); isExistArticle {
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