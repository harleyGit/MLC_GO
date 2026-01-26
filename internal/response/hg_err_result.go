/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-01-26 21:28:25
 * @LastEditors: Harley harelysoa@qq.com
 * @LastEditTime: 2026-01-26 22:45:21
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