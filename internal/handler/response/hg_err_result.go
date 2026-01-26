/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-26 21:28:25
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-26 21:28:28
 * @FilePath: /MLC_GO/internal/handler/response/hg_err_result.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
 package HGResponsePakcage
type HGErrorResult struct {
	Code int `json:"-"`
	Message string `json:"-"`
}

func (e HGErrorResult) ResponseCode() int  {
	return  e.Code
}

func (e HGErrorResult) ResponseMessage() string {
	return e.Message
}

/*

*/