/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-01 21:47:45
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-06 18:02:22
 * @FilePath: /MLC_GO/TestNotes/PracticeGenExample/routers/api/auth.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package api

import (
	"MLC_GO/TestNotes/GenPracticeExample/models"
	"MLC_GO/TestNotes/GenPracticeExample/pkg/e"
	"MLC_GO/TestNotes/GenPracticeExample/pkg/logging"
	"MLC_GO/TestNotes/GenPracticeExample/pkg/util"
	"net/http"

	"github.com/astaxie/beego/validation"
	"github.com/gin-gonic/gin"
)

type auth struct {
	/* 
		校验标签：valid:"Required; MaxSize(50)"

		valid 是 第三方数据验证库 govalidator 提供的标签，用于自动校验结构体字段。

			这段代码的 valid 标签定义了两个验证规则：
				Required：用户名是必填字段，不能为空。
				MaxSize(50)：用户名最大长度为 50 个字符，不能超过。
	*/
	Username string `valid:"Required; MaxSize(50)"`
	Password string `valid:"Required; MaxSize(50)"`
}

func GetAuth(c *gin.Context) {
	username := c.Query("username")
	password := c.Query("password")

	valid := validation.Validation{}
	a := auth{Username: username, Password: password}
	ok, _ := valid.Valid(&a)

	data := make(map[string]interface{})
	code := e.INVALID_PARAMS
	if ok {
		isExist := models.CheckAuth(username, password)
		if isExist {
			token, err := util.GenerateToken(username, password)
			if err != nil {
				code = e.ERROR_AUTH_TOKEN
			} else {
				data["token"] = token
				code  = e.SUCCESS
			}
		} else {
			code = e.ERROR_AUTH
		}
	} else {
		for _, err := range valid.Errors {
			logging.Info(err.Key, err.Message)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code": code,
		"msg": e.GetMsg(code),
		"data": data,
	})
}