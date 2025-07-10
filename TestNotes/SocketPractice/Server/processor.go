/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-07-10 18:55:14
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-07-10 20:01:43
 * @FilePath: /MLC_GO/TestNotes/SocketPractice/Server/processor.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package server

import (
	"MLC_GO/TestNotes/SocketPractice/common/message"
	"MLC_GO/pkg/logHG"
	"io"
	"net"
)

type Processor struct {
	Conn net.Conn
}

// 根据客户端发送消息种类不同，决定用哪个函数来处理
func (this *Processor) serverProcessMes(mes *message.Message) (err error) {

	logHG.DebugInfo("mes = ", mes)

	switch mes.Type {
	case message.LoginMesType:
		// 处理登录
		// 创建一个UserProcess实例对象
		up := &process2.UserProcess{
			Conn: this.Conn,
		}
		err = up.ServerProcessLogin(mes)
	case message.RegisterMesType:
		// 处理注册
		up := &process2.UserProcess{
			Conn: this.Conn,
		}
		err = up.ServerProcessRegister(mes)
	case message.SmsMesType:
		// 创建一个SmsProcess实例完成转发群聊消息
		smsProcess := &process2.SmsProcess{}
		smsProcess.SendGroupMes(mes)
	default:
		logHG.DebugInfo("消息类型不存在，无法处理.....")
	}
	return
}

func (this *Processor) process2() (err error) {

	// 循环的客户端发送的消息
	for {
		// 读取数据包，直接封装成一个函数readPkg(),返回Message, Err
		// 创建一个Transfer 实例完成读包任务
		tf := &utils.Transfer{
			Conn: this.Conn,
		}
		mes, err := tf.ReadPkg()
		if err != nil {
			if err == io.EOF {
				logHG.DebugInfo("客户端退出，服务器端也退出.....")
			} else {
				logHG.DebugInfo("readPkg err = ", err)
			}
		}
		err = this.serverProcessMes(&mes)
		if err !=  nil {
			return err
		}
	}
}
