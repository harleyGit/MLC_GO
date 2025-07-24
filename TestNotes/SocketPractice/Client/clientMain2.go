/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-07-24 14:27:29
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-07-24 14:36:51
 * @FilePath: /MLC_GO/TestNotes/SocketPractice/Client/ClientMain/clientMain2.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package clientmain

import (
	"MLC_GO/TestNotes/SocketPractice/Client/process"
	"MLC_GO/pkg/logHG"
	"fmt"
	"os"
)

var userId int     //表示用户id
var userPwd string //表示用户密码
var userName string 

func clientMain() {
	var key int     // 接受用户选择
	// var loop = true // 判断是否还继续显示菜单

	for true {
		logHG.DebugInfo("----------------------欢迎登录多人聊天系统--------------------")
		logHG.DebugInfo("\t\t\t 1 登录聊天室")
		logHG.DebugInfo("\t\t\t 2 注册用户")
		logHG.DebugInfo("\t\t\t 3 退出系统")
		logHG.DebugInfo("\t\t\t 请选择（1-3）:")

		fmt.Scanf("%d\n", &key)
		switch key {
		case 1:
			logHG.DebugInfo("登录聊天室")
			logHG.DebugInfo("请输入用户的ID")
			fmt.Scanf("%d\n", &userId)
			logHG.DebugInfo("请输入用户的密码")
			fmt.Scanf("%s\n", &userPwd)
			// 完成登录
			up := &process.UserProcess{}
			up.Login(userId, userPwd)
		case 2:
			logHG.DebugInfo("注册用户")
			logHG.DebugInfo("请输入用户的ID")
			fmt.Scanf("%d\n", &userId)
			logHG.DebugInfo("请输入用户的密码")
			fmt.Scanf("%s\n", &userPwd)
			logHG.DebugInfo("请输入用户名字(nickname):")
			fmt.Scanf("%s\n", &userName)
			// 完成登录
			up := &process.UserProcess{}
			up.Register(userId, userPwd, userName)
		case 3:
			logHG.DebugInfo("退出系统")
			os.Exit(0)
		default:
			logHG.DebugInfo("输入有误，请重新输入......")
		}
	}
}
