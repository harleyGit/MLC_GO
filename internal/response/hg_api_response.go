/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-28 20:05:36
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-28 20:56:39
 * @FilePath: /MLC_GO/internal/response/hg_api_response.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package HGResponsePakcage

import "net/http"

/* 基础响应结构【HTTP JSON容器】 */
type HGAPIResponseModel struct {
	Code      HGErrorCode `json:"code"`
	Message   string      `json:"message"`
	Result    interface{} `json:"result"`
	TID       string      `json:"tid"`
	Timestamp int64       `json:"timestamp"`
}

func Success(w http.ResponseWriter, r *http.Request, result HGResultModel) {
	WriteJSON(w, r, result)
}

func Fail(w http.ResponseWriter, r *http.Request, err HGErrorResult) {

	WriteJSON(w, r, err)
}
