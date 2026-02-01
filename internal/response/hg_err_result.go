/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-26 21:28:25
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-02-01 16:25:33
 * @FilePath: /MLC_GO/internal/handler/response/hg_err_result.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package HGResponsePakcage

// TODO: 名字改为HGResultCode
type HGErrorCode int

const (
	OKCode HGErrorCode = 200

	// 通用错误
	UnauthorizedCode   HGErrorCode = 101001
	InvalidParamCode   HGErrorCode = 100002
	ForbiddenCode      HGErrorCode = 100003
	NotFoundCode       HGErrorCode = 100004
	MethodNotAllowCode HGErrorCode = 100005

	MethodNotFoundCode HGErrorCode = 100000405

	// 业务错误
	UserNotFoundCode HGErrorCode = 404001
	UserRegisterFail HGErrorCode = 404002

	OrderNotFoundCode HGErrorCode = 405002

	// 系统错误
	InternalErrorCode HGErrorCode = 500001
)

type HGErrorResult struct {
	Code    HGErrorCode `json:"-"`
	Message string      `json:"-"`
}

var (
	OK            = HGErrorResult{OKCode, "success"}
	Unauthorized  = HGErrorResult{UnauthorizedCode, "unauthorized"}
	InvalidParam  = HGErrorResult{InvalidParamCode, "invalid param"}
	InternalError = HGErrorResult{InternalErrorCode, "internal server error"}
)

func NewErrResult(code HGErrorCode, msg string) *HGErrorResult {
	return &HGErrorResult{Code: code, Message: msg}
}

func (e *HGErrorResult) ErrMSG() string {
	return e.Message
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
