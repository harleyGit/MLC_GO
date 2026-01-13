/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-07-17 21:19:19
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-13 11:05:09
 * @FilePath: /MLC_GO/TestNotes/SocketPractice/Server/process/smsProcess.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package process2

import (
	"MLC_GO/TestNotes/SocketPractice/Server/utils"
	"MLC_GO/TestNotes/SocketPractice/common/message"
	"MLC_GO/internal/pkg/logHG"
	"encoding/json"
	"net"
)

type SmsProcess struct {
	// 后续可能使用字段
}

// 写方法转发消息
func (this *SmsProcess) SendGroupMes(mes *message.Message) {

	// 遍历服务器端的onlineUsers map[int] *UserProcess,
	// 将消息转发取出
	// 取出mes的内容 SmsMes
	var smsMes message.SmsMes
	err := json.Unmarshal([]byte(mes.Data), &smsMes)
	if err != nil {
		logHG.ErrInfo("json.Unmarshal err = ", err)
		return
	}
	data, err := json.Marshal(mes)
	if err != nil {
		logHG.ErrInfo("json.Marshal err:", err)
		return
	}

	for id, up := range userMgr.onlineUsers {
		//过虐掉自己，即不要再发送给自己
		if id == smsMes.UserId {
			continue
		}
		this.SendMesToEachOnlineUser(data, up.Conn)
	}
}

func (this *SmsProcess) SendMesToEachOnlineUser(data []byte, conn net.Conn) {
	//创建一个Transfer实例， 发送data
	tf := &utils.Transfer{
		Conn: conn,
	}
	err := tf.WritePkg(data)
	if err != nil {
		logHG.ErrInfo("转发消息失败 err = ", err)
	}
}
