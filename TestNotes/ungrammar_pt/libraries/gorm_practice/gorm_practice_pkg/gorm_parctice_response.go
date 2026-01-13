/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-20 17:06:13
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-13 11:11:57
 * @FilePath: /MLC_GO/TestNotes/ungrammar_pt/libraries/gorm_practice/gorm_practice_pkg/gorm_parctice_response.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package gorm_practice_pkg

import (
	"MLC_GO/internal/pkg/hg_response"

	"github.com/gin-gonic/gin"
)

type GormGin struct {
	GGin *gin.Context
}

type Response  struct {
	Code int `json:"code"`
	Msg string `json:"msg"`
	Data interface{} `json:"data"`	
}

func (g *GormGin) Response(httpCode, errCode int, data interface{}) {
	g.GGin.JSON(httpCode, Response{
		Code: errCode,
		Msg:  hg_response.GetMsg(errCode),
		Data: data,
	})
}