package gorm_practice_pkg

import (
	"MLC_GO/pkg/hg_response"

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