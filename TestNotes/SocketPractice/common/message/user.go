/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-07-10 19:41:54
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-07-10 19:44:06
 * @FilePath: /MLC_GO/TestNotes/SocketPractice/common/message/user.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package message

type User struct {
	UserId int `json:"userId"`
	UserPwd string `json:"userPwd"`
	UserName string `json: "userName"`
	UserStatus int `json:"userStatus"`//用户状态
	Sex string `json:"sex"`// 性别
}