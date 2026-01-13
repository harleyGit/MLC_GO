/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-07-21 14:53:07
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-13 11:01:54
 * @FilePath: /MLC_GO/TestNotes/SocketPractice/Client/ClientMain/clientMain.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package clientmain

import (
	"MLC_GO/internal/pkg/logHG"
	"fmt"
	"os"
)

var userId int //表示用户id
var userPwd string//表示用户密码

func clientMain() {
	var key int // 接受用户选择
	var loop = true // 判断是否还继续显示菜单

	for loop {
		logHG.DebugInfo("----------------------欢迎登录多人聊天系统--------------------")
		logHG.DebugInfo("\t\t\t 1 登录聊天室")
		logHG.DebugInfo("\t\t\t 2 注册用户")
		logHG.DebugInfo("\t\t\t 3 退出系统")
		logHG.DebugInfo("\t\t\t 请选择（1-3）:")

		fmt.Scanf("%d\n", &key)
		switch key {
		case 1: 
		logHG.DebugInfo("登录聊天室")
		loop = false
		case 2: 
		logHG.DebugInfo("注册用户")
		loop =false
		case 3:
			logHG.DebugInfo("退出系统")
			os.Exit(0)
		default:
			logHG.DebugInfo("输入有误，请重新输入......")
		}
	}

	// 增加用户输入,显示新的提示信息
	if  key == 1 {
		// 说明用户要登录
		logHG.DebugInfo("请输入用的ID")
		fmt.Scanf("%d\n", &userId)
		logHG.DebugInfo("请输入用户的密码")
		fmt.Scanf("%s\n", &userPwd)
		// 先把登录的函数，写到另一个文件，比如：login.go
		login(userId, userPwd)
	}else if key == 2 {
		logHG.DebugInfo("进行用户注册的逻辑.......")
	}
}

