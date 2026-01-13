/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-07-22 18:11:24
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-13 11:03:06
 * @FilePath: /MLC_GO/TestNotes/SocketPractice/common/message/process/userMgr.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package process

import (
	"MLC_GO/TestNotes/SocketPractice/Client/model"
	"MLC_GO/TestNotes/SocketPractice/common/message"
	"MLC_GO/internal/pkg/logHG"
)

// 客户端要维护的map
var onlineUsers map[int]*message.User = make(map[int]*message.User, 10)
var CurUser model.CurUser //我们在登录成功后，完成对CurUser初始化

// 在客户端显示当前在线用户
func outputOnlineUser() {

	logHG.DebugInfo("当前在线用户列表：")
	// 遍历onlineUsers
	for id, _ := range onlineUsers {
		// 如果不显示自己
		logHG.DebugInfo("用户ID：\t", id)
	}
}

// 处理返回的NotifyUserStatusMes
func updateUserStatus(notifyUserStatusMes *message.NotifyUserStatusMes) {
	
	// 适当优化
	user, ok := onlineUsers[notifyUserStatusMes.UserId]
	if !ok {//原来没有
		user = &message.User{
			UserId: notifyUserStatusMes.UserId,
		}
	}
	user.UserStatus = notifyUserStatusMes.Status
	onlineUsers[notifyUserStatusMes.UserId] = user

	outputOnlineUser()
}