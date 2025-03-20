/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-19 21:54:23
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-20 20:09:13
 * @FilePath: /MLC_GO/TestNotes/unfamiliar_grammar_practice/libraries/gorm_practice/gorm_practice_routers/gorm_router_api/gorm_user_api.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package gorm_router_api

import (
	"MLC_GO/TestNotes/unfamiliar_grammar_practice/libraries/gorm_practice/gorm_practice_models"
	"MLC_GO/TestNotes/unfamiliar_grammar_practice/libraries/gorm_practice/gorm_practice_pkg"
	"MLC_GO/TestNotes/unfamiliar_grammar_practice/libraries/gorm_practice/gorm_practice_service"
	"MLC_GO/pkg/hg_response"
	"MLC_GO/pkg/hglog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

//get请求，param 参数
func GetUserByUid(c *gin.Context) {

}

/* 表单插入数据
curl -X POST http://localhost:8080/api/user/addUser \
     -d "name=一剑飘雪🔥" \
     -d "age=27" \
     -d "sex=8" \
     -d "phone=17683838865"\
	 -d "day_of_the_beast=2010-01-01"
	
注意: day_of_the_beast必须有,否则 MySQL 在 严格模式（sql_mode 启用了 NO_ZERO_DATE）下，会禁止 0000-00-00 00:00:00 作为 datetime 值
*/
// post 请求， 普通 form 表单获取参数
func AddUser(gin *gin.Context) {
	newGin := gorm_practice_pkg.GormGin{GGin: gin}

	// 从 POST 请求 的 form-data 里获取 name 参数
	name := newGin.GGin.PostForm("name")
	age, _ := strconv.ParseInt(newGin.GGin.PostForm("age"), 10, 32)
	sex, _ := strconv.ParseInt(newGin.GGin.PostForm("sex"), 10, 8)
	phone := newGin.GGin.PostForm("phone")
	
	// birthday,_ := time.Parse("2006-01-02", newGin.GGin.PostForm("day_of_the_beast"))
	birthdayStr := newGin.GGin.PostForm("day_of_the_beast")
	loc, _ := time.LoadLocation("Asia/Shanghai")  // 加载中国时区
	birthday, _ := time.ParseInLocation("2006-01-02", birthdayStr, loc)

	userModel := gorm_practice_models.GormUser{	
		Id: 0,
		Name: name,
		Age: int32(age),	
		Sex: int8(sex),
		Phone: phone,
		Birthday: birthday,
	}
	err := gorm_practice_service.AddNewUser(userModel)
	if err != nil {	
		newGin.Response(http.StatusBadRequest, hg_response.INVALID_PARAMS, nil)
		return
	}
	newGin.Response(http.StatusOK, hg_response.SUCCESS, userModel)
}



