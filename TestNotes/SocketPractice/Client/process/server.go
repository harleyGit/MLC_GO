/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-07-21 20:02:59
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-13 11:02:36
 * @FilePath: /MLC_GO/TestNotes/SocketPractice/common/message/process/server.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package process

import (
	"MLC_GO/TestNotes/SocketPractice/Server/utils"
	"MLC_GO/TestNotes/SocketPractice/common/message"
	"MLC_GO/internal/pkg/logHG"
	"encoding/json"
	"fmt"
	"net"
	"os"
)

// 显示登录成功后的界面
func ShowMenu() {
	logHG.DebugInfo("----------恭喜xxx登录成功--------")
	logHG.DebugInfo("----------1. 显示在线用户列表--------")
	logHG.DebugInfo("----------2. 发送消息--------")
	logHG.DebugInfo("----------3. 信息列表--------")
	logHG.DebugInfo("----------4. 退出系统--------")
	logHG.DebugInfo("请选择（1-4）")
	var key int
	var content string

	// 因为我们总会使用到SmsProcess实例，因此我们将其定义在switch外部
	smsProcess := &SmsProcess{}
	fmt.Scanf("%d\n", &key)
	switch key {
	case 1:
		outputOnlineUser()
	case 2:
		logHG.DebugInfo("你想对大家说什么:)")
		fmt.Scanf("%s\n", &content)
		smsProcess.SendGroupMes(content)
	case 3:
		logHG.DebugInfo("信息列表")
	case 4:
		logHG.DebugInfo("你选择退出了系统.....")
		os.Exit(0)
	default:
		logHG.DebugInfo("你输入的选项不正确...")
	}
}

// 和服务器保持通讯
func serverProcessMes(conn net.Conn) {
	// 创建一个transfe实例，不停的读取服务器发送的消息
	tf := &utils.Transfer{
		Conn: conn,
	}
	for {
		logHG.DebugInfo("客户端正在等待读取服务器发送的消息")
		mes, err := tf.ReadPkg()
		if err != nil {
			logHG.ErrInfo("tf.ReadPkg err: ", err)
			return
		}

		// 如果读取到消息，又是下一步处理逻辑
		switch mes.Type {
		case message.NotifyUserStatusMesType: //有人上线了
			// 1.取出 .NotifyUserStatusMes
			var notifyUserStatusMes message.NotifyUserStatusMes
			json.Unmarshal([]byte(mes.Data), &notifyUserStatusMes)
			// 2.把这个用户的信息，状态保存到客户map[int]User中
			updateUserStatus(&notifyUserStatusMes)
			//处理
		case message.SmsMesType: // 有人群发消息
			outputGroupMes(&mes)
		default:
			logHG.DebugInfo("服务器端返回了未知消息类型")
		}
	}
}
