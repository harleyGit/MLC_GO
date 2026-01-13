/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-07-22 17:58:49
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-01-13 11:02:57
 * @FilePath: /MLC_GO/TestNotes/SocketPractice/common/message/process/smsProcess.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package process

import (
	"MLC_GO/TestNotes/SocketPractice/Client/utils"
	"MLC_GO/TestNotes/SocketPractice/common/message"
	"MLC_GO/internal/pkg/logHG"
	"encoding/json"
)

type SmsProcess struct {}

// 发送群聊消息
func (this *SmsProcess) SendGroupMes(content string) (err error) {

	// 1.创建一个Mes 
	var mes message.Message
	mes.Type = message.SmsMesType

	// 2.创建一个SmsMes实例 
	var smsMes message.SmsMes
	smsMes.Content = content //内容
	smsMes.UserId = CurUser.UserId
	smsMes.UserStatus = CurUser.UserStatus 

	// 3.序列化 smsMes 
	data, err := json.Marshal(smsMes)
	if err != nil {
		logHG.ErrInfo("SendGroupMes json.Marshal fail = ", err.Error())
		return
	}
	
	mes.Data = string(data)

	// 4.对mes 再次序列化 
	data, err = json.Marshal(mes)
	if err != nil {
		logHG.ErrInfo("SendGroupMes json.Marshal fail = ", err.Error())
		return
	}

	// 5. 将mes 发送给服务器
	tf := &utils.Transfer{
		Conn: CurUser.Conn,
	}

	//6.发送
	err = tf.WritePkg(data)
	if err != nil {
		logHG.ErrInfo("SendGroupMes err = ", err.Error())
		return
	}
	return
}