/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-27 18:18:26
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-28 20:45:14
 * @FilePath: /MLC_GO/TestNotes/handler_practice/hg_handler_test.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package HGPracticeHandlerPackage

import (
	HGMiddlewarePackage "MLC_GO/internal/interfaces/middleware"
	"MLC_GO/internal/pkg/logHG"
	UtilsPackage "MLC_GO/internal/pkg/utils"
	HGResponsePakcage "MLC_GO/internal/response"
	"net/http"
)

type HGPracticeHandler struct {
	Status int64 `json:"status"`
}

func NewPracticeTestHandler() *HGPracticeHandler {

	return &HGPracticeHandler{Status: 0}
}

func PracticeTestHandler() http.Handler {

	pt := NewPracticeTestHandler()
	practiceMux := http.NewServeMux()

	practiceMux.HandleFunc("/ok", pt.OkTestHandler)
	practiceMux.HandleFunc("/error", pt.ErrTestHandler)

	practiceTestHandler := HGMiddlewarePackage.JSONHeaderMiddleware(
		HGMiddlewarePackage.TIDMiddleware(practiceMux),
	)
	return practiceTestHandler
}

/*
  - 正常测试
    curl http://localhost:8080/test/ok

    {
    "code": 0,
    "message": "success💯",
    "result": {
    "user": "Master💎",
    "age": 25
    },
    "tid": "fe4c35afefa3b8d4d8102ffdf522a2a6",
    "timestamp": 1769518081566
    }%
*/
func (th *HGPracticeHandler) OkTestHandler(w http.ResponseWriter, r *http.Request) {

	tid := UtilsPackage.GetTID(r.Context())

	logHG.DebugFInfo("[TID=%s] 测试 业务逻辑开始", tid)

	resultModel := &HGResultPracticeModel{User: "Master1💎", Age: 25}

	HGResponsePakcage.WriteJSON(w, r, resultModel)
}

/*
  - 异常错误测试-panic
    curl http://localhost:8080/test/error

    {
    "code": 500,
    "message": "internal server error",
    "result": {},
    "tid": "7e10547599fb7e71fcf5007162d6171f",
    "timestamp": 1769518154830
    }
*/
func (th *HGPracticeHandler) ErrTestHandler(w http.ResponseWriter, r *http.Request) {

	tid := UtilsPackage.GetTID(r.Context())
	logHG.DebugFInfo("[TID=%s] 测试 业务逻辑开始", tid)

	// 模拟业务错误
	panic("database 连接失败☹️")
}
