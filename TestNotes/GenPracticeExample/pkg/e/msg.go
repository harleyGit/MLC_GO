/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-02-23 21:13:56
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-02-23 21:25:35
 * @FilePath: /MLC_GO/TestNotes/PracticeGenExample/pkg/e/msg.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */

package e


 var MsgFlags = map[int]string {
	SUCCESS : "ok",
	ERROR : "fail",
	INVALID_PARAMS : "请求参数错误",
	ERROR_EXIST_TAG : "已存在该标签名称",
	ERROR_NOT_EXIST_TAG : "该标签不存在",
	ERROR_NOT_EXIST_ARTICLE : "该文章不存在",
	ERROR_AUTH_CHECK_TOKEN_FAIL : "Token鉴权失败",
	ERROR_AUTH_CHECK_TOKEN_TIMEOUT : "Token已超时",
	ERROR_AUTH_TOKEN : "Token生成失败",
	ERROR_AUTH : "Token错误",
 } 

 func GetMsg(code int) string {
	msg, ok := MsgFlags[code]
	if ok {
		return msg
	}

	return MsgFlags[ERROR]
}