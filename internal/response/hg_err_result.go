/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-26 21:28:25
 * @LastEditors: Harley harelysoa@qq.com
 * @LastEditTime: 2026-01-28 22:26:59
 * @FilePath: /MLC_GO/internal/handler/response/hg_err_result.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package HGResponsePakcage

type HGErrorCode int

const (
	OKCode            HGErrorCode = 200
	UnauthorizedCode  HGErrorCode = 401001
	InvalidParamCode  HGErrorCode = 400001
	InternalErrorCode HGErrorCode = 500001
	UserNotFoundCode  HGErrorCode = 404001
	OrderNotFoundCode HGErrorCode = 404002
)

type HGErrorResult struct {
	Code    HGErrorCode `json:"-"`
	Message string    `json:"-"`
}

var (
  OK = HGErrorResult{OKCode, "success"}
  Unauthorized = HGErrorResult{UnauthorizedCode, "unauthorized"} 
  InvalidParam = HGErrorResult{InvalidParamCode, "invalid param"}
  InternalError = HGErrorResult{InternalErrorCode, "internal server error"}
)

func NewErrResult(code HGErrorCode, msg string) *HGErrorResult {
  return  &HGErrorResult{Code: code, Message: msg}
}

func (e HGErrorResult) ResponseCode() HGErrorCode {
	return e.Code
}

func (e HGErrorResult) ResponseMessage() string {
	return e.Message
}

/* 使用方式
HGResponsePakcage.WriteJSON(
    w,
    tid,
    response.ErrorResult{
        Code:    10001,
        Message: "手机号或密码错误",
    },
)


结果返回：
{
  "code": 10001,
  "message": "手机号或密码错误",
  "result": {},
  "tid": "...",
  "timestamp": ...
}

*/
