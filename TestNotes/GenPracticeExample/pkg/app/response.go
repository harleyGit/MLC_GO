/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-04 21:01:39
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-06 20:01:33
 * @FilePath: /MLC_GO/TestNotes/GenPracticeExample/pkg/app/response.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package app

import (
	"MLC_GO/TestNotes/GenPracticeExample/pkg/e"

	"github.com/gin-gonic/gin"
)

type Gin struct {
	C *gin.Context
}

type Response struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

// Response setting gin.JSON
func (g *Gin) Response(httpCode, errCode int, data interface{}) {
	g.C.JSON(httpCode, Response{
		Code: errCode,
		Msg:  e.GetMsg(errCode),
		Data: data,
	})
	return
}
