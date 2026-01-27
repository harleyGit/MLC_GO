/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-27 19:52:53
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-27 20:35:01
 * @FilePath: /MLC_GO/TestNotes/handler_practice/hg_result_test_model.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package HGPracticeHandlerPackage

import "net/http"

type HGResultPracticeModel struct {
	User string `json:"user"`
	Age  int64  `json:"age"`
}

func (ttm *HGResultPracticeModel) ResponseCode() int {
	return http.StatusOK
}

func (ttm *HGResultPracticeModel) ResponseMessage() string {
	return "success💯"
}
